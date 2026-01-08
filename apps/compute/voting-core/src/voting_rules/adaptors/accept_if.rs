//! AcceptIf adaptor module.

use tracing::instrument;

use crate::profile::Profile;
use crate::tie_breaker::RuleOutcome;
use crate::voting_rules::VotingRuleExec;

/// AcceptIf adaptor.
///
/// Accepts the candidate set if it meets a predicate F.
pub struct AcceptIf<V, F> {
    /// Voting rule to get the candidate set from.
    voting_rule: V,
    /// Predicate the would accept or reject the candidate set as a whole.
    predicate: F,
}

impl<V, F> AcceptIf<V, F> {
    /// Construct a new AcceptIf instance.
    pub fn new(voting_rule: V, predicate: F) -> Self {
        Self {
            voting_rule,
            predicate,
        }
    }
}

impl<V, F> VotingRuleExec for AcceptIf<V, F>
where
    V: VotingRuleExec,
    F: Fn(&RuleOutcome) -> bool,
{
    type Error = V::Error;

    #[instrument(skip(self, profile))]
    fn execute(&self, profile: &Profile) -> Result<RuleOutcome, Self::Error> {
        let outcome = self.voting_rule.execute(profile)?;
        tracing::debug!(?outcome, "Calculated outcome");

        if (self.predicate)(&outcome) {
            tracing::debug!("Predicate is true, accepting outcome");
            Ok(outcome)
        } else {
            tracing::debug!("Predicate is false, rejecting outcome");
            Ok(RuleOutcome::MultipleWinners(outcome.candidates()))
        }
    }
}

#[cfg(test)]
mod tests {
    use crate::profile::CandidateId;

    use super::*;
    use mockall::mock;

    mock! {
        pub SuccessfulVotingRule {

        }

        impl VotingRuleExec for SuccessfulVotingRule {
            type Error = ();

            fn execute(&self, profile: &Profile) -> Result<RuleOutcome, <Self as VotingRuleExec>::Error>;
        }
    }

    fn fake_profile() -> Profile {
        Profile::try_from(vec![vec![0, 2, 1]]).unwrap()
    }

    #[test]
    fn test_does_match_outcome() {
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
    fn test_doesnt_match_outcome() {
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
    fn test_voting_rule_error_is_propagated() {
        let mut mock = MockSuccessfulVotingRule::new();

        mock.expect_execute().return_const(Err(()));

        assert_eq!(
            Err(()),
            AcceptIf::new(mock, |outcome: &RuleOutcome| outcome.is_unique())
                .execute(&fake_profile())
        );
    }
}
