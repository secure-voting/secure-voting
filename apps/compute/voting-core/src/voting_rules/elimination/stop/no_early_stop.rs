//! No early stopper module.

use crate::{
    prelude::{Profile, RuleOutcome},
    scorer::Score,
    voting_rules::elimination::stop::EliminationStopCondition,
};

/// Don't stop early. Just returns false if asked whether to stop the elimination process.
#[derive(Debug, Clone, Copy)]
pub struct NoEarlyStop;

impl<S> EliminationStopCondition<S> for NoEarlyStop {
    fn should_stop(&self, _: &Score<S>, _: &RuleOutcome, _: &Profile) -> bool {
        false
    }

    fn new() -> Self {
        Self
    }
}
