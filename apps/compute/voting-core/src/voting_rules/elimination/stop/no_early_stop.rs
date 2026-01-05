//! No early stopper module.

use crate::{
    prelude::{Profile, RuleOutcome},
    voting_rules::elimination::stop::EliminationStopCondition,
};

/// Don't stop early. Just returns false if asked whether to stop the elimination process.
pub struct NoEarlyStop;

impl<S> EliminationStopCondition<S> for NoEarlyStop {
    fn should_stop(&self, _: &S, _: &RuleOutcome, _: &Profile) -> bool {
        false
    }
}
