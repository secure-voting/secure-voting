//! Majority elimination stop condition module.

use crate::{
    prelude::{Profile, RuleOutcome},
    voting_rules::elimination::stop::EliminationStopCondition,
};

/// Majority elimination stop condition type.
///
/// Checks whether to stop if any candidate has a strict majority of votes.
pub struct MajorityStop;

impl EliminationStopCondition<Vec<usize>> for MajorityStop {
    fn should_stop(&self, scores: &Vec<usize>, _: &RuleOutcome, profile: &Profile) -> bool {
        let total = profile.n_voters();

        scores.iter().any(|&s| s * 2 > total)
    }
}
