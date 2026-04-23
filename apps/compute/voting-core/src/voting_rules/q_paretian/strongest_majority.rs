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
    prelude::CandidateId,
    tie_breaker::RuleOutcome,
    voting_rules::{
        Final, Kind, Metrics, Protocol, Step, Summary, VotingRuleExec,
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

    fn execute(
        &self,
        profile: &Profile<RankingBallot>,
    ) -> Result<(RuleOutcome, Metrics, Protocol), Self::Error> {
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
                let winners: Vec<CandidateId> = survivors
                    .iter()
                    .map(|&i| profile.active_candidates()[i].clone())
                    .collect();

                return Ok((
                    RuleOutcome::from(winners.clone()),
                    Metrics::builder()
                        .summary(
                            Summary::builder()
                                .total_ballots(n)
                                .valid_ballots(n)
                                .invalid_ballots(0)
                                .candidates_count(m)
                                .winner_count(winners.len())
                                .committee_size(0)
                                .rounds_count(1)
                                .build(),
                        )
                        .build(),
                    Protocol::builder()
                        .kind(Kind::SingleStep)
                        .steps(vec![
                            Step::builder()
                                .step(1)
                                .title("Round 1".to_owned())
                                .action("declare_winner".to_owned())
                                .build(),
                        ])
                        .r#final(
                            Final::builder()
                                .winner_ids(winners.iter().map(ToString::to_string).collect())
                                .build(),
                        )
                        .build(),
                ));
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
