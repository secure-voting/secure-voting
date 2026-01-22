//! Fallback adaptor module.

use std::fmt::Debug;
use thiserror::Error;
use tracing::instrument;

use crate::{prelude::Profile, tie_breaker::RuleOutcome, voting_rules::VotingRuleExec};

/// A fallback adaptor.
///
/// If the primary rule can't decide a single winner, a fallback rule will be used to determine the winner instead.
#[derive(Debug, Clone, Copy)]
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

    #[instrument(skip(self, profile))]
    fn execute(&self, profile: &Profile) -> Result<RuleOutcome, Self::Error> {
        match self.primary.execute(profile) {
            Ok(RuleOutcome::UniqueWinner(winner)) => {
                tracing::debug!("Primary rule returned a unique winner");
                Ok(RuleOutcome::UniqueWinner(winner))
            }
            Ok(RuleOutcome::MultipleWinners(_)) => {
                tracing::debug!("Primary rule can't decide winner, running fallback");
                self.fallback
                    .execute(profile)
                    .map_err(FallbackError::FallbackError)
            }
            Err(e) => Err(FallbackError::PrimaryError(e)),
        }
    }

    fn create_default() -> Self
    where
        Self: Sized,
    {
        Fallback::default()
    }
}

impl<R1, R2> Default for Fallback<R1, R2>
where
    R1: VotingRuleExec,
    R2: VotingRuleExec,
{
    fn default() -> Self {
        Self {
            primary: R1::create_default(),
            fallback: R2::create_default(),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::{
        profile::CandidateId, profile::Profile, tie_breaker::RuleOutcome,
        voting_rules::VotingRuleExec,
    };
    use mockall::mock;

    mock! {
        pub VotingRule {}

        impl VotingRuleExec for VotingRule {
            type Error = &'static str;

            fn execute(&self, profile: &Profile) -> Result<RuleOutcome, <Self as VotingRuleExec>::Error>;
            fn create_default() -> Self where Self: Sized;
        }
    }

    fn fake_profile() -> Profile {
        Profile::try_from(vec![vec![0, 1]])
            .expect("Profile is constructed incorrectly, revise test example")
    }

    #[test]
    fn primary_unique_winner_short_circuits() {
        let mut primary = MockVotingRule::new();
        let mut fallback = MockVotingRule::new();

        primary
            .expect_execute()
            .times(1)
            .return_const(Ok(RuleOutcome::UniqueWinner(CandidateId::new(0))));

        fallback.expect_execute().times(0);

        let rule = Fallback::new(primary, fallback);

        let result = rule.execute(&fake_profile());

        assert_eq!(
            result.expect("The rule shouldn't fail"),
            RuleOutcome::UniqueWinner(CandidateId::new(0))
        );
    }

    #[test]
    fn primary_multiple_winners_triggers_fallback() {
        let mut primary = MockVotingRule::new();
        let mut fallback = MockVotingRule::new();

        primary
            .expect_execute()
            .times(1)
            .return_const(Ok(RuleOutcome::MultipleWinners(vec![
                CandidateId::new(0),
                CandidateId::new(1),
            ])));

        fallback
            .expect_execute()
            .times(1)
            .return_const(Ok(RuleOutcome::UniqueWinner(CandidateId::new(1))));

        let rule = Fallback::new(primary, fallback);

        let result = rule.execute(&fake_profile());

        assert_eq!(
            result.expect("The rule shouldn't fail"),
            RuleOutcome::UniqueWinner(CandidateId::new(1))
        );
    }

    #[test]
    fn primary_error_is_propagated_and_fallback_not_called() {
        let mut primary = MockVotingRule::new();
        let mut fallback = MockVotingRule::new();

        primary
            .expect_execute()
            .times(1)
            .return_const(Err("primary failed"));

        fallback.expect_execute().times(0);

        let rule = Fallback::new(primary, fallback);

        let result = rule.execute(&fake_profile());

        assert!(matches!(
            result,
            Err(FallbackError::PrimaryError("primary failed"))
        ));
    }

    #[test]
    fn fallback_error_is_wrapped_correctly() {
        let mut primary = MockVotingRule::new();
        let mut fallback = MockVotingRule::new();

        primary
            .expect_execute()
            .times(1)
            .return_const(Ok(RuleOutcome::MultipleWinners(vec![
                CandidateId::new(0),
                CandidateId::new(1),
            ])));

        fallback
            .expect_execute()
            .times(1)
            .return_const(Err("fallback failed"));

        let rule = Fallback::new(primary, fallback);

        let result = rule.execute(&fake_profile());

        assert!(matches!(
            result,
            Err(FallbackError::FallbackError("fallback failed"))
        ));
    }
}
