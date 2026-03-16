//! Majority elimination stop condition module.

use crate::{
    models::ranking::RankingBallot,
    prelude::{Profile, RuleOutcome},
    scorer::Score,
    voting_rules::elimination::stop::EliminationStopCondition,
};

/// Majority elimination stop condition type.
///
/// Checks whether to stop if any candidate has a strict majority of votes.
#[derive(Debug, Clone, Copy)]
pub struct MajorityStop;

impl EliminationStopCondition<Vec<usize>, RankingBallot> for MajorityStop {
    fn should_stop(
        &self,
        scores: &Score<Vec<usize>>,
        _: &RuleOutcome,
        profile: &Profile<RankingBallot>,
    ) -> bool {
        let total = profile.n_voters();

        scores.iter().any(|(s, _)| s * 2 > total)
    }

    fn new() -> Self {
        Self
    }
}

#[cfg(test)]
mod tests {
    use crate::prelude::CandidateId;

    use super::*;
    use test_case::test_case;

    #[test_case(vec![1, 1, 1], vec![vec![0, 1, 2], vec![2, 0, 1], vec![1, 2, 0]], false; "no majority winner")]
    #[test_case(vec![2, 1, 0], vec![vec![0, 1, 2], vec![0, 1, 2], vec![1, 2, 0]], true; "majority winner")]
    fn test_majority_stop(scores: Vec<usize>, votes: Vec<Vec<usize>>, result: bool) {
        assert_eq!(
            result,
            MajorityStop.should_stop(
                &Score::new(scores, &(0..3).map(CandidateId::new).collect::<Vec<_>>()),
                &RuleOutcome::MultipleWinners(vec![]),
                &votes
                    .try_into()
                    .expect("Profile is constructed incorrectly, revise test example")
            )
        );
    }
}
