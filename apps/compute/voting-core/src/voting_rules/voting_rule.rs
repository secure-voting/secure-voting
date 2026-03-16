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
    voting_rules::VotingRuleExec,
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
pub struct VotingRule<S, D, T, Ballot> {
    /// A scorer instance.
    scorer: S,
    /// A decider instance.
    decider: D,
    /// A tie-breaker instance.
    tiebreaker: T,
    /// Phantom marker on the Ballot type.
    _ballot_type: PhantomData<Ballot>,
}

/// Helper result type returned from the [`super::VotingRuleExec::execute`] method of [`VotingRule`] struct.
///
/// Allows the method to fail in each of 3 steps, propagating the returned error up.
pub type VotingRuleResult<S, D, T, Ballot> = Result<
    RuleOutcome,
    VotingRuleError<
        <S as Scorer<Ballot>>::Error,
        <D as Decider>::Error,
        <T as TieBreaker<Ballot>>::Error,
    >,
>;

impl<S, D, T, Ballot> VotingRule<S, D, T, Ballot>
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
        }
    }

    /// Run the constructed pipeline.
    ///
    /// Returns an error if any of the steps didn't succeed.
    #[instrument(skip(self, profile), ret)]
    fn run(&self, profile: &Profile<Ballot>) -> VotingRuleResult<S, D, T, Ballot> {
        let scores = self
            .scorer
            .compute_score(profile)
            .map_err(VotingRuleError::ScoringError)?;

        let candidates = self
            .decider
            .decide(&scores)
            .map_err(VotingRuleError::DecisionError)?;
        tracing::debug!(?candidates, "Calculated a set of winners");

        self.tiebreaker
            .tie_break(&candidates, profile)
            .map_err(VotingRuleError::TieBreakError)
    }
}

impl<S: Scorer<Ballot, Output = D::Input>, D: Decider, T: TieBreaker<Ballot>, Ballot>
    VotingRuleExec<Ballot> for VotingRule<S, D, T, Ballot>
{
    type Error = VotingRuleError<S::Error, D::Error, T::Error>;

    fn execute(&self, profile: &Profile<Ballot>) -> Result<RuleOutcome, Self::Error> {
        self.run(profile)
    }

    fn create_default() -> Self
    where
        Self: Sized,
    {
        Self::default()
    }
}

impl<S: Scorer<Ballot, Output = D::Input>, D: Decider, T: TieBreaker<Ballot>, Ballot> Default
    for VotingRule<S, D, T, Ballot>
{
    fn default() -> Self {
        Self {
            scorer: S::new(),
            decider: D::new(),
            tiebreaker: T::new(),
            _ballot_type: PhantomData,
        }
    }
}
