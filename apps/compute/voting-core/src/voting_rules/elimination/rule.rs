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
        Metrics, Protocol, VotingRuleExec,
        elimination::{criterion::EliminationCriterion, stop::EliminationStopCondition},
    },
};

/// Elimination strategy.
///
/// If the the whole pipeline can't decide a unique winner,
/// then the lowest candidate is eliminated and the process
/// is rerun until the winner is unique.
#[derive(Debug)]
pub struct Elimination<S, E, D, T, Ballot, Stop> {
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
}

impl<S, E, D, T, Stop, Ballot> Elimination<S, E, D, T, Ballot, Stop> {
    /// Construct an eliminator rule instance.
    pub fn new(scorer: S, eliminator: E, decider: D, tiebreaker: T, stop: Stop) -> Self {
        Self {
            scorer,
            eliminator,
            decider,
            tiebreaker,
            stop,
            _ballot_type: PhantomData,
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

impl<S, E, D, T, Stop> VotingRuleExec<RankingBallot>
    for Elimination<S, E, D, T, RankingBallot, Stop>
where
    S: Scorer<RankingBallot, Output = D::Input>,
    E: EliminationCriterion<Score = S::Output>,
    D: Decider,
    T: TieBreaker<RankingBallot>,
    Stop: EliminationStopCondition<S::Output, RankingBallot>,
    <D as Decider>::Input: PartialOrd + Ord + Clone + Into<f64>,
{
    type Error = EliminationRuleError<S::Error, D::Error, T::Error, CandidateRemovalError>;

    #[instrument(skip(self, profile), ret)]
    fn execute(
        &self,
        profile: &Profile<RankingBallot>,
    ) -> Result<(RuleOutcome, Metrics, Protocol), Self::Error> {
        let mut current_profile = profile.clone();

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

            if self.stop.should_stop(&scores, &outcome, profile) {
                tracing::debug!("Stopping condition met, finishing elimination rounds");
                return Ok(outcome);
            }

            let to_remove = self.eliminator.eliminate(&scores);
            tracing::debug!(?to_remove, "Removing candidates");

            if to_remove.is_empty() {
                tracing::debug!("Removed candidate set is empty, stopping on undecided state");
                return Ok(RuleOutcome::MultipleWinners(vec![]));
            }

            current_profile = current_profile
                .remove_candidates(to_remove)
                .map_err(EliminationRuleError::CandidateRemovalError)?;
            tracing::debug!("Candidates left: {:?}", current_profile.active_candidates());

            if current_profile.active_candidates().len() == 1 {
                tracing::debug!("Unique winner found, stopping elimination");
                let winner = current_profile.active_candidates()[0];
                return Ok(RuleOutcome::UniqueWinner(winner));
            }
        }
    }

    fn create_default() -> Self
    where
        Self: Sized,
    {
        Self::default()
    }
}

impl<S, E, D, T, Ballot, Stop> Default for Elimination<S, E, D, T, Ballot, Stop>
where
    S: Scorer<Ballot, Output = D::Input>,
    E: EliminationCriterion<Score = S::Output>,
    D: Decider,
    T: TieBreaker<Ballot>,
    Stop: EliminationStopCondition<S::Output, Ballot>,
{
    fn default() -> Self {
        Self {
            scorer: S::new(),
            eliminator: E::new(),
            decider: D::new(),
            tiebreaker: T::new(),
            stop: Stop::new(),
            _ballot_type: PhantomData,
        }
    }
}
