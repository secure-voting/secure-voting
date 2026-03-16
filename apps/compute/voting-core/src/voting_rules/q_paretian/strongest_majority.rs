//! Strongest q-Paretian Simple Majority Rule module.
//!
//! This module defines the [`SimpleMajorityRule`] struct.

/// Strongest q-Paretian Simple Majority Rule defined as per paper\[1\].
///
/// Citation:
/// 1. Aleskerov, F., Kurbanov, E. Degree of manipulability of social choice procedures. In: Alkan, A., Aliprantis, C.D., Yannelis, N.C. (eds) Current Trends in Economics. Studies in Economic Theory, v.8. 1999, Springer, Berlin, Heidelberg. doi: 10.1007/978-3-662-03750-8_2
use itertools::Itertools;
use rayon::prelude::*;

use crate::{
    models::{profile::Profile, ranking::RankingBallot},
    tie_breaker::RuleOutcome,
    voting_rules::{
        VotingRuleExec,
        q_paretian::{QParetianError, build_pos},
    },
};

/// Strongest q-Paretian Simple Majority Rule defined as per paper\[1\].
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
            let survivors: Vec<usize> = (0..m)
                .into_par_iter()
                .filter(|&x| {
                    !(0..n).combinations(r).any(|coalition| {
                        (0..m).any(|y| {
                            if y == x {
                                return false;
                            }

                            let count =
                                coalition.iter().filter(|&&i| pos[i][y] < pos[i][x]).count();

                            count >= coalition.len() - q
                        })
                    })
                })
                .collect();

            if !survivors.is_empty() {
                return Ok(RuleOutcome::from(survivors));
            }
        }

        unreachable!("q = m-1 always produces candidates")
    }

    fn create_default() -> Self
    where
        Self: Sized,
    {
        SimpleMajorityRule
    }
}
