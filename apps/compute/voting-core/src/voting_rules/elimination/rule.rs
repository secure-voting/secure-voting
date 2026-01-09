//! Elimination Rule module.
//!
//! This module defines the [`Elimination`] struct.

use std::fmt::Debug;

use thiserror::Error;
use tracing::instrument;

use crate::{
    decider::Decider,
    profile::{CandidateRemovalError, Profile},
    scorer::Scorer,
    tie_breaker::{RuleOutcome, TieBreaker},
    voting_rules::{
        VotingRuleExec,
        elimination::{
            criterion::EliminationCriterion,
            stop::{EliminationStopCondition, no_early_stop::NoEarlyStop},
        },
    },
};

/// Elimination strategy.
///
/// If the the whole pipeline can't decide a unique winner,
/// then the lowest candidate is eliminated and the process
/// is rerun until the winner is unique.
pub struct Elimination<S, E, D, T, Stop = NoEarlyStop> {
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
}

impl<S, E, D, T, Stop> Elimination<S, E, D, T, Stop> {
    /// Construct an eliminator rule instance.
    pub fn new(scorer: S, eliminator: E, decider: D, tiebreaker: T, stop: Stop) -> Self {
        Self {
            scorer,
            eliminator,
            decider,
            tiebreaker,
            stop,
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

impl<S, E, D, T, Stop> VotingRuleExec for Elimination<S, E, D, T, Stop>
where
    S: Scorer<Output = D::Input>,
    E: EliminationCriterion<Score = S::Output>,
    D: Decider,
    T: TieBreaker,
    Stop: EliminationStopCondition<S::Output>,
{
    type Error = EliminationRuleError<S::Error, D::Error, T::Error, CandidateRemovalError>;

    #[instrument(skip(self, profile), ret)]
    fn execute(&self, profile: &Profile) -> Result<RuleOutcome, Self::Error> {
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
            tracing::debug!(?candidates, "Elected a set of candidates");

            let outcome = self
                .tiebreaker
                .tie_break(&candidates, &current_profile)
                .map_err(EliminationRuleError::TieBreakError)?;
            tracing::debug!(?outcome, "Calculated a final outcome");

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
        }
    }
}

impl<S, E, D, T, Stop> Default for Elimination<S, E, D, T, Stop>
where
    S: Scorer<Output = D::Input>,
    E: EliminationCriterion<Score = S::Output>,
    D: Decider,
    T: TieBreaker,
    Stop: EliminationStopCondition<S::Output>,
{
    fn default() -> Self {
        Self {
            scorer: S::new(),
            eliminator: E::new(),
            decider: D::new(),
            tiebreaker: T::new(),
            stop: Stop::new(),
        }
    }
}
