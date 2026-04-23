//! Elimination Rule module.
//!
//! This module defines the [`Elimination`] struct.

use std::{fmt::Debug, marker::PhantomData};

use thiserror::Error;
use tracing::instrument;

use crate::{
    decider::Decider,
    models::{
        profile::{CandidateRemovalError, Profile},
        ranking::RankingBallot,
    },
    scorer::Scorer,
    tie_breaker::{RuleOutcome, TieBreaker},
    voting_rules::{
        Final, Kind, Metrics, Protocol, RoundSize, Series, Step, Summary, ToScore,
        VotingRuleExec,
        elimination::{criterion::EliminationCriterion, stop::EliminationStopCondition},
    },
};

/// Elimination strategy.
///
/// If the the whole pipeline can't decide a unique winner,
/// then the lowest candidate is eliminated and the process
/// is rerun until the winner is unique.
#[derive(Debug)]
pub struct Elimination<S, E, D, T, Ballot, Stop, U> {
    /// The scorer step of the pipeline.
    scorer: S,
    /// The eliminator step of the pipeline.
    eliminator: E,
    /// The decider step of the pipeline.
    decider: D,
    /// The tie breaker step of the pipeline.
    tiebreaker: T,
    /// The elimination stop checker.
    stop: Stop,
    /// Ballot type marker.
    _ballot_type: PhantomData<Ballot>,
    /// Score type marker.
    _scorer_type: PhantomData<U>,
}

impl<S, E, D, T, Stop, Ballot, U> Elimination<S, E, D, T, Ballot, Stop, U> {
    /// Construct an eliminator rule instance.
    pub fn new(scorer: S, eliminator: E, decider: D, tiebreaker: T, stop: Stop) -> Self {
        Self {
            scorer,
            eliminator,
            decider,
            tiebreaker,
            stop,
            _ballot_type: PhantomData,
            _scorer_type: PhantomData,
        }
    }
}

/// Elimination strategy rule error type.
#[derive(Error, Debug, PartialEq, Eq)]
pub enum EliminationRuleError<SE, DE, TE, RE>
where
    SE: Debug,
    DE: Debug,
    TE: Debug,
    RE: Debug,
{
    /// Returned when the scorer errors.
    #[error(transparent)]
    ScoringError(SE),
    /// Returned when the candidate can't be removed errors.
    #[error(transparent)]
    CandidateRemovalError(RE),
    /// Returned when the decider errors
    #[error(transparent)]
    DecisionError(DE),
    /// Returned when the tiebreaker errors.
    #[error(transparent)]
    TieBreakError(TE),
}

impl<'a, U, S, E, D, T, Stop> VotingRuleExec<RankingBallot>
    for Elimination<S, E, D, T, RankingBallot, Stop, U>
where
    S: Scorer<RankingBallot, Output = D::Input>,
    E: EliminationCriterion<Score = S::Output>,
    D: Decider,
    T: TieBreaker<RankingBallot>,
    Stop: EliminationStopCondition<S::Output, RankingBallot>,
    <D as Decider>::Input: AsRef<[U]>,
    U: 'a + ToScore,
{
    type Error = EliminationRuleError<S::Error, D::Error, T::Error, CandidateRemovalError>;

    #[allow(clippy::unwrap_used)]
    #[instrument(skip(self, profile), ret)]
    fn execute(
        &self,
        profile: &Profile<RankingBallot>,
    ) -> Result<(RuleOutcome, Metrics, Protocol), Self::Error> {
        let mut current_profile = profile.clone();

        let mut steps = vec![];
        let mut round_sizes = vec![];
        let summary = Summary::builder()
            .total_ballots(profile.n_voters())
            .valid_ballots(profile.n_voters())
            .invalid_ballots(0)
            .candidates_count(profile.n_candidates())
            .committee_size(0);

        loop {
            let scores = self
                .scorer
                .compute_score(&current_profile)
                .map_err(EliminationRuleError::ScoringError)?;

            let candidates = self
                .decider
                .decide(&scores)
                .map_err(EliminationRuleError::DecisionError)?;
            tracing::debug!(?candidates, "Elected a set of");

            let outcome = self
                .tiebreaker
                .tie_break(&candidates, &current_profile)
                .map_err(EliminationRuleError::TieBreakError)?;
            tracing::debug!(?outcome, "Calculated an");

            let mut cur_step = Step::builder()
                .step(steps.len() + 1)
                .title(format!("Round {}", steps.len() + 1))
                .action("recount".into())
                .scores(
                    scores
                        .iter()
                        .map(|(score, cand)| score.to_score(cand.to_string(), cand.get_name().to_owned()))
                        .collect(),
                )
                .build();

            let cur_round_size = RoundSize::builder()
                .round(round_sizes.len() + 1)
                .remaining_candidates(profile.active_candidates().len())
                .build();
            round_sizes.push(cur_round_size);

            if self.stop.should_stop(&scores, &outcome, profile) {
                tracing::debug!("Stopping condition met, finishing elimination rounds");

                cur_step.set_action("declare_winner");
                steps.push(cur_step);
                let protocol = Protocol::builder()
                    .kind(Kind::EliminationRounds)
                    .steps(steps.clone())
                    .r#final(
                        Final::builder()
                            .winner_ids(
                                outcome
                                    .candidates()
                                    .iter()
                                    .map(ToString::to_string)
                                    .collect(),
                            )
                            .build(),
                    )
                    .build();
                let summary = summary
                    .winner_count(outcome.candidates().len())
                    .rounds_count(steps.len())
                    .build();
                let metrics = Metrics::builder()
                    .summary(summary)
                    .series(
                        Series::builder()
                            .round_sizes(round_sizes)
                            .candidate_scores_final(
                                scores
                                    .iter()
                                    .map(|(score, cand)| {
                                        score.to_score(cand.to_string(), cand.get_name().to_owned())
                                    })
                                    .collect(),
                            )
                            .build(),
                    )
                    .build();

                return Ok((outcome, metrics, protocol));
            }

            let to_remove = self.eliminator.eliminate(&scores);
            tracing::debug!(?to_remove, "Removing candidates");

            cur_step.set_eliminated(&to_remove);

            if to_remove.is_empty() {
                tracing::debug!("Removed candidate set is empty, stopping on undecided state");
                steps.push(cur_step);
                let protocol = Protocol::builder()
                    .kind(Kind::EliminationRounds)
                    .steps(steps.clone())
                    .r#final(Final::builder().winner_ids(vec![]).build())
                    .build();
                let summary = summary.winner_count(0).rounds_count(steps.len()).build();
                let metrics = Metrics::builder()
                    .summary(summary)
                    .series(
                        Series::builder()
                            .round_sizes(round_sizes)
                            .candidate_scores_final(
                                scores
                                    .iter()
                                    .map(|(score, cand)| {
                                        score.to_score(cand.to_string(), cand.get_name().to_owned())
                                    })
                                    .collect(),
                            )
                            .build(),
                    )
                    .build();

                return Ok((RuleOutcome::MultipleWinners(vec![]), metrics, protocol));
            }

            current_profile = current_profile
                .remove_candidates(to_remove)
                .map_err(EliminationRuleError::CandidateRemovalError)?;
            tracing::debug!("Candidates left: {:?}", current_profile.active_candidates());

            cur_step.set_remaining(current_profile.active_candidates());

            if current_profile.active_candidates().len() == 1 {
                tracing::debug!("Unique winner found, stopping elimination");
                let winner = &current_profile.active_candidates()[0];

                steps.push(cur_step);
                let protocol = Protocol::builder()
                    .kind(Kind::EliminationRounds)
                    .steps(steps.clone())
                    .r#final(
                        Final::builder()
                            .winner_ids(vec![winner.to_string()])
                            .build(),
                    )
                    .build();
                let summary = summary.winner_count(1).rounds_count(steps.len()).build();
                let metrics = Metrics::builder()
                    .summary(summary)
                    .series(
                        Series::builder()
                            .round_sizes(round_sizes)
                            .candidate_scores_final(
                                scores
                                    .iter()
                                    .map(|(score, cand)| {
                                        score.to_score(cand.to_string(), cand.get_name().to_owned())
                                    })
                                    .collect(),
                            )
                            .build(),
                    )
                    .build();

                return Ok((RuleOutcome::UniqueWinner(winner.clone()), metrics, protocol));
            }

            steps.push(cur_step);
        }
    }

    fn create_default() -> Self
    where
        Self: Sized,
    {
        Self::default()
    }
}

impl<'a, S, E, D, T, Ballot, Stop, U> Default for Elimination<S, E, D, T, Ballot, Stop, U>
where
    S: Scorer<Ballot, Output = D::Input>,
    E: EliminationCriterion<Score = S::Output>,
    D: Decider,
    T: TieBreaker<Ballot>,
    Stop: EliminationStopCondition<S::Output, Ballot>,
    <D as Decider>::Input: AsRef<[U]>,
    U: 'a + ToScore,
{
    fn default() -> Self {
        Self {
            scorer: S::new(),
            eliminator: E::new(),
            decider: D::new(),
            tiebreaker: T::new(),
            stop: Stop::new(),
            _ballot_type: PhantomData,
            _scorer_type: PhantomData,
        }
    }
}
