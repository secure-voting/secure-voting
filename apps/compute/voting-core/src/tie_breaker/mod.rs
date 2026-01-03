//! Tie-breaking strategies for resolving ties among selected winners.
//!
//! This module defines the [`TieBreaker`] trait and provides pre-packaged implementations.

use std::fmt::Debug;

use crate::profile::{CandidateId, Profile};

pub mod fallthrough;

/// The outcome of performing a tie-breaker.
pub enum RuleOutcome {
    /// A single winner is chosen.
    UniqueWinner(CandidateId),
    /// This tie-breaker can't decide a winner
    MultipleWinners(Vec<CandidateId>),
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
