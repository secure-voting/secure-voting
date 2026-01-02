//! Fallback adaptor module.

use std::fmt::Debug;
use thiserror::Error;

use crate::{tie_breaker::RuleOutcome, voting_rules::VotingRuleExec};

/// A fallback adaptor.
///
/// If the primary rule can't decide a single winner, a fallback rule will be used to determine the winner instead.
pub struct Fallback<R1, R2> {
    /// Primary voting Rule
    primary: R1,
    /// Fallback voting Rule
    fallback: R2,
}

impl<R1, R2> Fallback<R1, R2> {
    /// Construct a Fallback adaptor from the primary and fallback rules.
    pub fn new(primary: R1, fallback: R2) -> Self {
        Self { primary, fallback }
    }
}

/// Fallback adaptor error.
///
/// Has two variants:
///
/// - Primary
/// - Fallback
///
/// Variants represent which part of execution errored out.
#[derive(Error, Debug)]
pub enum FallbackError<P: Debug, F: Debug> {
    /// Returned when the primary voting rule returns an error itself.
    #[error("primary rule failed: {0:?}")]
    PrimaryError(P),
    /// Returned when the fallback voting rule returns an error itself.
    #[error("fallback rule failed: {0:?}")]
    FallbackError(F),
}

impl<R1, R2> VotingRuleExec for Fallback<R1, R2>
where
    R1: VotingRuleExec,
    R2: VotingRuleExec,
{
    type Error = FallbackError<R1::Error, R2::Error>;

    fn execute(&self, profile: &crate::profile::Profile) -> Result<RuleOutcome, Self::Error> {
        match self.primary.execute(profile) {
            Ok(RuleOutcome::Winner(winner)) => Ok(RuleOutcome::Winner(winner)),
            Ok(RuleOutcome::Undecided) => self
                .fallback
                .execute(profile)
                .map_err(FallbackError::FallbackError),
            Err(e) => Err(FallbackError::PrimaryError(e)),
        }
    }
}
