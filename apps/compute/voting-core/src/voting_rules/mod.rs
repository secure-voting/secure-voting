//! Different methods of constructing a voting rule.
//!
//! This module defines the trait [`VotingRuleExec`] and pre-defined voting rule implementations.

use crate::profile::{CandidateId, Profile};

pub mod voting_rule;

/// Trait for all the voting rules, simple and complex ones.
pub trait VotingRuleExec {
    /// Returned if the voting pipeline can't be completed.
    type Error;

    /// Run the constructed pipeline.
    fn run_pipeline(&self, profile: &Profile) -> Result<CandidateId, Self::Error>;
}
