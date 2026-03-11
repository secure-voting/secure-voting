//! Strong q-Paretian Simple Majority Rule module.
//!
//! This module defines the [`SimpleMajorityRule`] struct.

use itertools::Itertools;
use rayon::prelude::*;

use crate::{
    models::{profile::Profile, ranking::RankingBallot},
    tie_breaker::RuleOutcome,
    voting_rules::{
        VotingRuleExec,
        q_paretian::{QParetianError, build_pos, t_i_q_intersection},
    },
};

/// Strong q-Paretian Simple Majority Rule defined as per paper\[1\].
///
/// Citation:
/// 1. Aleskerov, F., Kurbanov, E. Degree of manipulability of social choice procedures. In: Alkan, A., Aliprantis, C.D., Yannelis, N.C. (eds) Current Trends in Economics. Studies in Economic Theory, v.8. 1999, Springer, Berlin, Heidelberg. doi: 10.1007/978-3-662-03750-8_2
pub struct SimpleMajorityRule<const LIMIT: usize>;

impl<const LIMIT: usize> VotingRuleExec<RankingBallot> for SimpleMajorityRule<LIMIT> {
    type Error = QParetianError;

    fn execute(&self, profile: &Profile<RankingBallot>) -> Result<RuleOutcome, Self::Error> {
        let n = profile.n_voters();
        let m = profile.n_candidates();
        let r = n / 2 + 1;

        if n > LIMIT {
            return Err(QParetianError::CombinatorialExplosion {
                limit: LIMIT,
                supplied: n,
            });
        }

        let pos = build_pos(profile);

        for q in 0..m {
            let mut result = (0..n)
                .combinations(r)
                .par_bridge()
                .map(|coalition| t_i_q_intersection(&coalition, q, &pos))
                .reduce(
                    std::vec::Vec::new,
                    |mut a: Vec<usize>, intersection: Vec<usize>| {
                        a.extend(intersection);
                        a
                    },
                );

            result.sort_unstable();
            result.dedup();
            if !result.is_empty() {
                return Ok(RuleOutcome::from(result));
            }
        }
        unreachable!("Q being m will always produce at least 1 candidate")
    }

    fn create_default() -> Self
    where
        Self: Sized,
    {
        SimpleMajorityRule
    }
}

#[allow(clippy::unwrap_used)]
#[allow(clippy::expect_used)]
#[cfg(test)]
mod tests {
    use crate::prelude::CandidateId;

    use super::*;
    use test_case::test_case;

    fn cand_ids(value: Vec<usize>) -> Vec<CandidateId> {
        value.into_iter().map(CandidateId::new).collect()
    }

    #[test_case(vec![vec![0], vec![0]], 0; "single candidate")]
    #[test_case(vec![vec![1, 0, 2]], 1; "degenerate majority")]
    #[test_case(vec![vec![0, 1, 2], vec![0, 2, 1], vec![0, 1, 2]], 0; "unanimous winner")]
    fn unique_winner(voters: Vec<Vec<usize>>, winner: usize) {
        let profile = Profile::try_from(voters)
            .expect("Profile was created incorrectly, revise text example");

        let result = SimpleMajorityRule::<30>.execute(&profile);
        assert!(matches!(result, Ok(RuleOutcome::UniqueWinner(_))));
        assert_eq!(result.unwrap().candidates(), vec![CandidateId::new(winner)]);
    }

    #[test_case(vec![vec![0, 1, 2], vec![1, 2, 0], vec![2, 0, 1]], vec![0, 1, 2]; "no majority agreement")]
    #[test_case(vec![vec![0, 1, 2], vec![1, 0, 2]], vec![0, 1]; "early q=1 detection")]
    #[test_case(vec![vec![0, 1, 2, 3], vec![1, 2, 0, 3], vec![2, 0, 1, 3]], vec![0, 1, 2]; "early q=2 detection")]
    fn multiple_winner(voters: Vec<Vec<usize>>, winners: Vec<usize>) {
        let profile = Profile::try_from(voters)
            .expect("Profile was created incorrectly, revise text example");

        let result = SimpleMajorityRule::<30>.execute(&profile);
        assert!(matches!(result, Ok(RuleOutcome::MultipleWinners(_))));
        assert_eq!(result.unwrap().candidates(), cand_ids(winners));
    }

    #[test]
    fn combinatorial_explosion_filter() {
        let voters = vec![vec![0, 1, 2]; 40];
        let profile = Profile::try_from(voters)
            .expect("Profile was created incorrectly, revise test example");

        let result = SimpleMajorityRule::<30>.execute(&profile);
        assert!(matches!(
            result,
            Err(QParetianError::CombinatorialExplosion {
                limit: _,
                supplied: _
            })
        ));
    }
}
