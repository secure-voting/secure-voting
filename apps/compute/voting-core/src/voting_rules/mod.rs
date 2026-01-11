//! Different methods of constructing a voting rule.
//!
//! This module defines the trait [`VotingRuleExec`] and pre-defined voting rule implementations.

use std::fmt::Debug;

use crate::{profile::Profile, tie_breaker::RuleOutcome};

pub mod adaptors;
pub mod elimination;
pub mod voting_rule;

pub mod anti_plurality;
pub mod approval;
pub mod black;
pub mod borda;
pub mod coombs;
pub mod copeland;
pub mod hare;
pub mod inverse_borda;
pub mod minmax;
pub mod nanson;
pub mod plurality;
pub mod practical_condorcet;
pub mod simpson;

/// Trait for all the voting rules, simple and complex ones.
pub trait VotingRuleExec: Default {
    /// Returned if the voting pipeline can't be completed.
    type Error: Debug;

    /// Run the constructed pipeline.
    fn execute(&self, profile: &Profile) -> Result<RuleOutcome, Self::Error>;
}
