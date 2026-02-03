//! No early stopper module.

use std::marker::PhantomData;

use crate::{
    prelude::{Profile, RuleOutcome},
    scorer::Score,
    voting_rules::elimination::stop::EliminationStopCondition,
};

/// Don't stop early. Just returns false if asked whether to stop the elimination process.
#[derive(Debug, Clone, Copy)]
pub struct NoEarlyStop<Ballot> {
    /// Ballot type marker.
    _ballot_type: PhantomData<Ballot>,
}

impl<S, Ballot> EliminationStopCondition<S, Ballot> for NoEarlyStop<Ballot> {
    fn should_stop(&self, _: &Score<S>, _: &RuleOutcome, _: &Profile<Ballot>) -> bool {
        false
    }

    fn new() -> Self {
        Self {
            _ballot_type: PhantomData,
        }
    }
}
