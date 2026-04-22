//! The require unique adaptor module.

use std::{fmt::Debug, marker::PhantomData};

use thiserror::Error;
use tracing::instrument;

use crate::{
    models::profile::Profile,
    tie_breaker::RuleOutcome,
    voting_rules::{Metrics, Protocol, VotingRuleExec},
};

/// Require unique adaptor.
///
/// Explicitly ensure that there is a single winner.
#[derive(Debug, Clone, Copy)]
pub struct RequireUnique<R, Ballot> {
    /// The rule to ensure a single winner in.
    rule: R,
    /// Ballot type marker.
    _ballot_type: PhantomData<Ballot>,
}

impl<R, Ballot> RequireUnique<R, Ballot> {
    /// Construct a `RequireUnique` adaptor from given rule.
    pub fn new(rule: R) -> Self {
        Self {
            rule,
            _ballot_type: PhantomData,
        }
    }
}

/// Require unique error type.
///
/// There are 2 ways the `RequireUnique`'s [`super::super::VotingRuleExec::execute`] can fail:
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

impl<R: VotingRuleExec<Ballot>, Ballot> VotingRuleExec<Ballot> for RequireUnique<R, Ballot> {
    type Error = RequireUniqueError<R::Error>;

    #[instrument(skip(self, profile))]
    fn execute(
        &self,
        profile: &Profile<Ballot>,
    ) -> Result<(RuleOutcome, Metrics, Protocol), Self::Error> {
        match self.rule.execute(profile)? {
            outcome @ (RuleOutcome::UniqueWinner(_), _, _) => {
                tracing::debug!("Rule returned a unique winner");
                Ok(outcome)
            }
            (RuleOutcome::MultipleWinners(_), _, _) => {
                tracing::error!("Multiple winners detected, returning error");
                Err(RequireUniqueError::NotUnique)
            }
        }
    }

    fn create_default() -> Self
    where
        Self: Sized,
    {
        Self::default()
    }
}

impl<R: VotingRuleExec<Ballot>, Ballot> Default for RequireUnique<R, Ballot> {
    fn default() -> Self {
        Self {
            rule: R::create_default(),
            _ballot_type: PhantomData,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::models::{candidate_id::CandidateId, ranking::RankingBallot};
    use mockall::mock;

    mock! {
        pub VotingRule {

        }

        impl VotingRuleExec<RankingBallot> for VotingRule {
            type Error = ();

            fn execute(&self, profile: &Profile<RankingBallot>) -> Result<RuleOutcome, <Self as VotingRuleExec<RankingBallot>>::Error>;
            fn create_default() -> Self where Self: Sized;
        }
    }

    fn fake_profile() -> Profile<RankingBallot> {
        Profile::try_from(vec![vec![0, 2, 1]])
            .expect("Profile is constructed incorrectly, revise test example.")
    }

    #[test]
    fn unique_winner_propagation() {
        let mut mock = MockVotingRule::new();

        mock.expect_execute()
            .times(1)
            .return_const(Ok(RuleOutcome::UniqueWinner(CandidateId::new(1))));

        assert_eq!(
            RuleOutcome::UniqueWinner(CandidateId::new(1)),
            RequireUnique::new(mock)
                .execute(&fake_profile())
                .expect("Shouldn't fail on a profile with a clear winner")
        );
    }

    #[test]
    fn error_propagation_from_rule() {
        let mut mock = MockVotingRule::new();

        mock.expect_execute().times(1).return_const(Err(()));

        assert!(matches!(
            RequireUnique::new(mock).execute(&fake_profile()),
            Err(RequireUniqueError::RuleError(()))
        ));
    }

    #[test]
    fn error_non_unique() {
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
