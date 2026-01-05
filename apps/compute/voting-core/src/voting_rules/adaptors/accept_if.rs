//! AcceptIf adaptor module.

use crate::prelude::{Profile, RuleOutcome, VotingRuleExec};

/// AcceptIf adaptor.
///
/// Accepts the candidate set if it meets a predicate F.
pub struct AcceptIf<V, F> {
    /// Voting rule to get the candidate set from.
    voting_rule: V,
    /// Predicate the would accept or reject the candidate set as a whole.
    predicate: F,
}

impl<V, F> VotingRuleExec for AcceptIf<V, F>
where
    V: VotingRuleExec,
    F: Fn(&RuleOutcome) -> bool,
{
    type Error = V::Error;

    fn execute(&self, profile: &Profile) -> Result<RuleOutcome, Self::Error> {
        let outcome = self.voting_rule.execute(profile)?;

        if (self.predicate)(&outcome) {
            Ok(outcome)
        } else {
            Ok(RuleOutcome::MultipleWinners(outcome.candidates()))
        }
    }
}
