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
            outcome @ RuleOutcome::UniqueWinner(_) => Ok(outcome),
            RuleOutcome::MultipleWinners(_) => Err(RequireUniqueError::NotUnique),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::profile::CandidateId;
    use mockall::mock;

    mock! {
        pub VotingRule {

        }

        impl VotingRuleExec for VotingRule {
            type Error = ();

            fn execute(&self, profile: &Profile) -> Result<RuleOutcome, <Self as VotingRuleExec>::Error>;
        }
    }

    fn fake_profile() -> Profile {
        Profile::try_from(vec![vec![0, 2, 1]]).unwrap()
    }

    #[test]
    fn test_unique_winner_propagation() {
        let mut mock = MockVotingRule::new();

        mock.expect_execute()
            .times(1)
            .return_const(Ok(RuleOutcome::UniqueWinner(CandidateId::new(1))));

        assert_eq!(
            RuleOutcome::UniqueWinner(CandidateId::new(1)),
            RequireUnique::new(mock).execute(&fake_profile()).unwrap()
        );
    }

    #[test]
    fn test_error_propagation_from_rule() {
        let mut mock = MockVotingRule::new();

        mock.expect_execute().times(1).return_const(Err(()));

        assert!(matches!(
            RequireUnique::new(mock).execute(&fake_profile()),
            Err(RequireUniqueError::RuleError(()))
        ));
    }

    #[test]
    fn test_error_non_unique() {
        let mut mock = MockVotingRule::new();

        mock.expect_execute()
            .times(1)
            .return_const(Ok(RuleOutcome::MultipleWinners(vec![
                CandidateId::new(0),
                CandidateId::new(1),
            ])));

        assert!(matches!(
            RequireUnique::new(mock).execute(&fake_profile()),
            Err(RequireUniqueError::NotUnique)
        ));
    }
}
