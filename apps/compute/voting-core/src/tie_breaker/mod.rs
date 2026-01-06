//! Tie-breaking strategies for resolving ties among selected winners.
//!
//! This module defines the [`TieBreaker`] trait and provides pre-packaged implementations.

use std::fmt::Debug;

use crate::profile::{CandidateId, Profile};

pub mod fallthrough;

/// The outcome of performing a tie-breaker.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum RuleOutcome {
    /// A single winner is chosen.
    UniqueWinner(CandidateId),
    /// This tie-breaker can't decide a winner
    MultipleWinners(Vec<CandidateId>),
}

impl RuleOutcome {
    /// Return the candidates of this outcome.
    pub fn candidates(&self) -> Vec<CandidateId> {
        match self {
            RuleOutcome::UniqueWinner(candidate_id) => vec![*candidate_id],
            RuleOutcome::MultipleWinners(candidate_ids) => candidate_ids.clone(),
        }
    }

    /// Check whether the winner is unique.
    pub fn is_unique(&self) -> bool {
        match self {
            RuleOutcome::UniqueWinner(_) => true,
            RuleOutcome::MultipleWinners(_) => false,
        }
    }
}

/// Resolves ties between selected winners to determine a single absolute winner.
pub trait TieBreaker {
    /// Error returned when the tie break can't be performed.
    ///
    /// Use [`std::convert::Infallible`] if this step cannot fail.
    type Error: Debug;

    /// Resolves a tie among the given candidates using the provided voting profile.
    /// Returns the chosen winner or an error if tie-breaking cannot be performed.
    fn tie_break(
        &self,
        candidates: &[CandidateId],
        profile: &Profile,
    ) -> Result<RuleOutcome, Self::Error>;
}
