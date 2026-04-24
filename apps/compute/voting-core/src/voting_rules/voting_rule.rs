//! Voting rule with the 3 steps.
//!
//! Allows to modularly combine a scorer, decider and tie-breaker of choice.
//!
//! This module defines the [`VotingRule`] struct as well as its [`VotingRuleError`] error type.

use std::{fmt::Debug, marker::PhantomData};
use thiserror::Error;
use tracing::instrument;

use crate::{
    decider::Decider,
    models::profile::Profile,
    scorer::Scorer,
    tie_breaker::{RuleOutcome, TieBreaker},
    voting_rules::{
        Final, Kind, Metrics, Protocol, RoundSize, Series, Step, Summary, ToScore, VotingRuleExec,
    },
};

/// `VotingRule` error type.
///
/// A voting rule can fail in each of its 3 steps, hence
/// this error type has 3 variants, one for each step.
///
/// Errors are propagated transparently, without adding new information.
#[derive(Error, Debug)]
pub enum VotingRuleError<SE, DE, TE>
where
    SE: Debug,
    DE: Debug,
    TE: Debug,
{
    /// Returned when an error occurs in the scoring step.
    #[error(transparent)]
    ScoringError(SE),
    /// Returned when an error occurs in the decision step.
    #[error(transparent)]
    DecisionError(DE),
    /// Returned when an error occurs in the tie-break step.
    #[error(transparent)]
    TieBreakError(TE),
}

/// Voting rule type.
///
/// Represents the full pipeline of voting:
///
/// 1. Scorer - transforms the raw vote data into score data for further analysis
/// 2. Decider - chooses a set of winners depending on the score information
/// 3. `TieBreaker` - chooses an absolute winner from the selected set
#[derive(Debug, Clone, Copy)]
pub struct VotingRule<S, D, T, Ballot, U> {
    /// A scorer instance.
    scorer: S,
    /// A decider instance.
    decider: D,
    /// A tie-breaker instance.
    tiebreaker: T,
    /// Phantom marker on the Ballot type.
    _ballot_type: PhantomData<Ballot>,
    /// Scorer inner value type marker.
    _score_type: PhantomData<U>,
}

/// Helper result type returned from the [`super::VotingRuleExec::execute`] method of [`VotingRule`] struct.
///
/// Allows the method to fail in each of 3 steps, propagating the returned error up.
pub type VotingRuleResult<S, D, T, Ballot> = Result<
    (RuleOutcome, Metrics, Protocol),
    VotingRuleError<
        <S as Scorer<Ballot>>::Error,
        <D as Decider>::Error,
        <T as TieBreaker<Ballot>>::Error,
    >,
>;

impl<S, D, T, Ballot, U> VotingRule<S, D, T, Ballot, U>
where
    S: Scorer<Ballot, Output = D::Input>,
    D: Decider,
    T: TieBreaker<Ballot>,
{
    /// Construct a new `VotingRule` from its 3 components.
    pub fn new(scorer: S, decider: D, tiebreaker: T) -> Self {
        Self {
            scorer,
            decider,
            tiebreaker,
            _ballot_type: PhantomData,
            _score_type: PhantomData,
        }
    }

    /// Run the constructed pipeline.
    ///
    /// Returns an error if any of the steps didn't succeed.
    #[allow(clippy::unwrap_used)]
    #[instrument(skip(self, profile), ret)]
    fn run<'a>(&self, profile: &Profile<Ballot>) -> VotingRuleResult<S, D, T, Ballot>
    where
        U: 'a + ToScore,
        <D as Decider>::Input: AsRef<[U]>,
    {
        let n = profile.n_voters();
        let m = profile.n_candidates();

        let scores = self
            .scorer
            .compute_score(profile)
            .map_err(VotingRuleError::ScoringError)?;

        let candidates = self
            .decider
            .decide(&scores)
            .map_err(VotingRuleError::DecisionError)?;
        tracing::debug!(?candidates, "Calculated a set of winners");

        let results = self
            .tiebreaker
            .tie_break(&candidates, profile)
            .map_err(VotingRuleError::TieBreakError)?;
        let winners = results.candidates();

        let scores_array: Vec<_> = scores
            .iter()
            .map(|(score, cand)| score.to_score(cand.to_string(), cand.get_name().to_owned()))
            .collect();

        let metrics = Metrics::builder()
            .summary(
                Summary::builder()
                    .total_ballots(n)
                    .valid_ballots(n)
                    .invalid_ballots(0)
                    .candidates_count(m)
                    .winner_count(winners.len())
                    .committee_size(0)
                    .rounds_count(1)
                    .build(),
            )
            .series(
                Series::builder()
                    .candidate_scores_final(scores_array.clone())
                    .round_sizes(vec![
                        RoundSize::builder()
                            .round(1)
                            .remaining_candidates(winners.len())
                            .build(),
                    ])
                    .build(),
            )
            .build();

        let protocol = Protocol::builder()
            .kind(Kind::SingleStep)
            .steps(vec![
                Step::builder()
                    .step(1)
                    .title("Round 1".to_owned())
                    .action("declare_winner".to_owned())
                    .remaining_candidate_ids(winners.iter().map(ToString::to_string).collect())
                    .scores(scores_array)
                    .build(),
            ])
            .r#final(
                Final::builder()
                    .winner_ids(winners.iter().map(ToString::to_string).collect())
                    .build(),
            )
            .build();

        Ok((results, metrics, protocol))
    }
}

impl<U, S: Scorer<Ballot, Output = D::Input>, D: Decider, T: TieBreaker<Ballot>, Ballot>
    VotingRuleExec<Ballot> for VotingRule<S, D, T, Ballot, U>
where
    <D as Decider>::Input: AsRef<[U]>,
    U: ToScore,
{
    type Error = VotingRuleError<S::Error, D::Error, T::Error>;

    fn execute(
        &self,
        profile: &Profile<Ballot>,
    ) -> Result<(RuleOutcome, Metrics, Protocol), Self::Error> {
        self.run(profile)
    }

    fn create_default() -> Self
    where
        Self: Sized,
    {
        Self::default()
    }
}

impl<U, S: Scorer<Ballot, Output = D::Input>, D: Decider, T: TieBreaker<Ballot>, Ballot> Default
    for VotingRule<S, D, T, Ballot, U>
where
    <D as Decider>::Input: AsRef<[U]>,
    U: ToScore,
{
    fn default() -> Self {
        Self {
            scorer: S::new(),
            decider: D::new(),
            tiebreaker: T::new(),
            _ballot_type: PhantomData,
            _score_type: PhantomData,
        }
    }
}
