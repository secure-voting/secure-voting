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

#[cfg(test)]
mod tests {
    use super::*;
    use test_case::test_case;

    #[test_case(vec![1, 1, 1], vec![vec![0, 1, 2], vec![2, 0, 1], vec![1, 2, 0]], false; "no majority winner")]
    #[test_case(vec![2, 1, 0], vec![vec![0, 1, 2], vec![0, 1, 2], vec![1, 2, 0]], true; "majority winner")]
    fn test_majority_stop(scores: Vec<usize>, votes: Vec<Vec<usize>>, result: bool) {
        assert_eq!(
            result,
            MajorityStop.should_stop(
                &scores,
                &RuleOutcome::MultipleWinners(vec![]),
                &votes.try_into().unwrap()
            )
        );
    }
}
