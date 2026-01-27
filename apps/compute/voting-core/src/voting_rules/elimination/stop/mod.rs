//! Elimination stop condition module.
//!
//! This module defines the [`EliminationStopCondition`] trait.

use crate::profile::Profile;
use crate::scorer::Score;
use crate::tie_breaker::RuleOutcome;

pub mod majority_stop;
pub mod no_early_stop;

/// Trait that encodes whether to stop the elimination process here.
pub trait EliminationStopCondition<S> {
    /// Checks whether it is already time to stop the elimination process.
    fn should_stop(&self, scores: &Score<S>, outcome: &RuleOutcome, profile: &Profile) -> bool;

    /// Construct a new Stopper.
    fn new() -> Self;
}
