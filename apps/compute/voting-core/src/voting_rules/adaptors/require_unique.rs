//! The require unique adaptor module.

use std::fmt::Debug;

use thiserror::Error;

use crate::{profile::Profile, tie_breaker::RuleOutcome, voting_rules::VotingRuleExec};

/// Require unique adaptor.
///
/// Explicitly ensure that there is a single winner.
pub struct RequireUnique<R> {
    /// The rule to ensure a single winner in.
    rule: R,
}

impl<R> RequireUnique<R> {
    /// Construct a RequireUnique adaptor from given rule.
    pub fn new(rule: R) -> Self {
        Self { rule }
    }
}

/// Require unique error type.
///
/// There are 2 ways the RequireUnique's [`execute`] can fail:
///
/// 1. There is no unique winner
/// 2. An error occured when choosing a set of winners
#[derive(Error, Debug)]
pub enum RequireUniqueError<RE: Debug> {
    /// No unique winner can be decided.
    #[error("no unique winner")]
    NotUnique,
    /// An error occured when choosing a set of winners.
    #[error(transparent)]
    RuleError(#[from] RE),
}

impl<R: VotingRuleExec> VotingRuleExec for RequireUnique<R> {
    type Error = RequireUniqueError<R::Error>;

    fn execute(&self, profile: &Profile) -> Result<RuleOutcome, Self::Error> {
        match self.rule.execute(profile)? {
            outcome @ RuleOutcome::Winner(_) => Ok(outcome),
            RuleOutcome::Undecided => Err(RequireUniqueError::NotUnique),
        }
    }
}
