//! `AcceptIf` adaptor module.

use std::marker::PhantomData;

use tracing::instrument;

use crate::models::profile::Profile;
use crate::tie_breaker::RuleOutcome;
use crate::voting_rules::{Metrics, Protocol, VotingRuleExec};

/// `AcceptIf` adaptor.
///
/// Accepts the candidate set if it meets a predicate.
pub struct AcceptIf<V, Ballot> {
    /// Voting rule to get the candidate set from.
    voting_rule: V,
    /// Predicate the would accept or reject the candidate set as a whole.
    predicate: Box<dyn Fn(&RuleOutcome) -> bool>,
    /// Ballot marker type.
    _ballot_type: PhantomData<Ballot>,
}

impl<V, Ballot> AcceptIf<V, Ballot> {
    /// Construct a new `AcceptIf` instance.
    pub fn new(voting_rule: V, predicate: impl Fn(&RuleOutcome) -> bool + 'static) -> Self {
        Self {
            voting_rule,
            predicate: Box::new(predicate),
            _ballot_type: PhantomData,
        }
    }
}

impl<V, Ballot> VotingRuleExec<Ballot> for AcceptIf<V, Ballot>
where
    V: VotingRuleExec<Ballot>,
{
    type Error = V::Error;

    #[instrument(skip(self, profile))]
    fn execute(
        &self,
        profile: &Profile<Ballot>,
    ) -> Result<(RuleOutcome, Metrics, Protocol), Self::Error> {
        let outcome = self.voting_rule.execute(profile)?;
        tracing::debug!(?outcome, "Calculated outcome");

        if (self.predicate)(&outcome.0) {
            tracing::debug!("Predicate is true, accepting outcome");
            Ok(outcome)
        } else {
            tracing::debug!("Predicate is false, rejecting outcome");
            Ok(RuleOutcome::MultipleWinners(outcome.candidates()))
        }
    }

    fn create_default() -> Self
    where
        Self: Sized,
    {
        AcceptIf::default()
    }
}

impl<V, Ballot> Default for AcceptIf<V, Ballot>
where
    V: VotingRuleExec<Ballot>,
{
    fn default() -> Self {
        Self {
            voting_rule: V::create_default(),
            predicate: Box::new(|_| true),
            _ballot_type: PhantomData,
        }
    }
}

#[cfg(test)]
mod tests {
    use crate::models::{candidate_id::CandidateId, ranking::RankingBallot};

    use super::*;
    use mockall::mock;

    mock! {
        pub SuccessfulVotingRule {

        }

        impl VotingRuleExec<RankingBallot> for SuccessfulVotingRule {
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
    fn does_match_outcome() {
        let mut mock = MockSuccessfulVotingRule::new();

        mock.expect_execute()
            .return_const(Ok(RuleOutcome::UniqueWinner(CandidateId::new(1))));

        assert_eq!(
            Ok(RuleOutcome::UniqueWinner(CandidateId::new(1))),
            AcceptIf::new(mock, |outcome: &RuleOutcome| outcome.is_unique())
                .execute(&fake_profile())
        );
    }

    #[test]
    fn doesnt_match_outcome() {
        let mut mock = MockSuccessfulVotingRule::new();

        mock.expect_execute()
            .return_const(Ok(RuleOutcome::MultipleWinners(vec![
                CandidateId::new(1),
                CandidateId::new(2),
            ])));

        assert_eq!(
            Ok(RuleOutcome::MultipleWinners(vec![
                CandidateId::new(1),
                CandidateId::new(2)
            ])),
            AcceptIf::new(mock, |outcome: &RuleOutcome| outcome.is_unique())
                .execute(&fake_profile())
        );
    }

    #[test]
    fn voting_rule_error_is_propagated() {
        let mut mock = MockSuccessfulVotingRule::new();

        mock.expect_execute().return_const(Err(()));

        assert_eq!(
            Err(()),
            AcceptIf::new(mock, |outcome: &RuleOutcome| outcome.is_unique())
                .execute(&fake_profile())
        );
    }
}
