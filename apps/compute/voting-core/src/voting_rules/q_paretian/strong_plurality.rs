//! Strong q-Paretian Plurality Rule module.
//!
//! This module defines the [`SimplePluralityRule`] struct.

/// Strong q-Paretian Plurality Rule defined as per paper\[1\].
///
/// Citation:
/// 1. Aleskerov, F., Kurbanov, E. Degree of manipulability of social choice procedures. In: Alkan, A., Aliprantis, C.D., Yannelis, N.C. (eds) Current Trends in Economics. Studies in Economic Theory, v.8. 1999, Springer, Berlin, Heidelberg. doi: 10.1007/978-3-662-03750-8_2
#[derive(Debug)]
pub struct SimplePluralityRule<const LIMIT: usize>;

use itertools::Itertools;
use rayon::prelude::*;

use crate::{
    models::{profile::Profile, ranking::RankingBallot},
    prelude::CandidateId,
    tie_breaker::RuleOutcome,
    voting_rules::{
        Final, Kind, Metrics, Protocol, Step, Summary, ToScore, VotingRuleExec,
        q_paretian::{QParetianError, build_pos, t_i_q_intersection},
    },
};
impl<const LIMIT: usize> VotingRuleExec<RankingBallot> for SimplePluralityRule<LIMIT> {
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
            let counts = (0..n)
                .combinations(r)
                .par_bridge()
                .map(|coalition| t_i_q_intersection(&coalition, q, &pos))
                .fold(
                    || vec![0usize; m],
                    |mut acc, intersection| {
                        for c in intersection {
                            acc[c] += 1;
                        }
                        acc
                    },
                )
                .reduce(
                    || vec![0usize; m],
                    |mut a, b| {
                        for i in 0..m {
                            a[i] += b[i];
                        }
                        a
                    },
                );

            let Some(&max) = counts.iter().max() else {
                continue;
            };

            if max > 0 {
                let winners: Vec<usize> = counts
                    .iter()
                    .enumerate()
                    .filter_map(|(i, &c)| if c == max { Some(i) } else { None })
                    .collect();
                let winners: Vec<CandidateId> = winners
                    .iter()
                    .map(|&i| profile.active_candidates()[i].clone())
                    .collect();
                let scores = counts
                    .iter()
                    .zip(profile.active_candidates().iter())
                    .map(|(score, cand)| (*score as f64).to_score(cand.to_string(), cand.get_name().to_owned()))
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
                                .tie_detected(winners.len() > 1)
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
                                .remaining_candidate_ids(profile.active_candidates().iter().map(ToString::to_string).collect())
                                .scores(scores)
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
        unreachable!("Q being m will always produce at least 1 candidate")
    }

    fn create_default() -> Self
    where
        Self: Sized,
    {
        SimplePluralityRule
    }
}

impl<const LIMIT: usize> Default for SimplePluralityRule<LIMIT> {
    fn default() -> Self {
        Self {}
    }
}

#[allow(clippy::unwrap_used)]
#[allow(clippy::expect_used)]
#[cfg(test)]
mod tests {
    use crate::prelude::CandidateId;

    use super::*;
    use test_case::test_case;

    #[test_case(vec![vec![0], vec![0]], vec!["A".into()], (0, "A"); "single candidate")]
    #[test_case(vec![vec![1, 0, 2]], vec!["A".into(), "B".into(), "C".into()], (1, "B"); "degenerate majority")]
    #[test_case(vec![vec![0, 1, 2], vec![0, 2, 1], vec![0, 1, 2]], vec!["A".into(), "B".into(), "C".into()], (0, "A"); "unanimous winner")]
    fn unique_winner(voters: Vec<Vec<usize>>, names: Vec<String>, winner: (usize, &str)) {
        let profile = Profile::try_from((voters, names))
            .expect("Profile was created incorrectly, revise text example");

        let result = SimplePluralityRule::<30>
            .execute(&profile)
            .expect("Unexpected error")
            .0;
        assert!(matches!(result, RuleOutcome::UniqueWinner(_)));
        assert_eq!(
            result.candidates(),
            vec![CandidateId::new(winner.0, winner.1)]
        );
    }

    #[test_case(
        vec![vec![0, 1, 2], vec![1, 2, 0], vec![2, 0, 1]],
        vec!["A".into(), "B".into(), "C".into()],
        vec![(0, "A"), (1, "B"), (2, "C")];
        "no majority agreement"
    )]
    #[test_case(
        vec![vec![0, 1, 2], vec![1, 0, 2]],
        vec!["A".into(), "B".into(), "C".into()],
        vec![(0, "A"), (1, "B")];
        "early q=1 detection"
    )]
    #[test_case(
        vec![vec![0, 1, 2, 3], vec![1, 2, 0, 3], vec![2, 0, 1, 3]],
        vec!["A".into(), "B".into(), "C".into(), "D".into()],
        vec![(0, "A"), (1, "B"), (2, "C")];
        "early q=2 detection"
    )]
    fn multiple_winner(voters: Vec<Vec<usize>>, names: Vec<String>, winners: Vec<(usize, &str)>) {
        let profile = Profile::try_from((voters, names))
            .expect("Profile was created incorrectly, revise text example");

        let result = SimplePluralityRule::<30>
            .execute(&profile)
            .expect("Unexpected error")
            .0;

        assert!(matches!(result, RuleOutcome::MultipleWinners(_)));

        let expected: Vec<_> = winners
            .into_iter()
            .map(|(i, name)| CandidateId::new(i, name))
            .collect();

        assert_eq!(result.candidates(), expected);
    }

    #[test]
    fn combinatorial_explosion_filter() {
        let voters = vec![vec![0, 1, 2]; 40];
        let names = vec!["A".into(), "B".into(), "C".into()];

        let profile = Profile::try_from((voters, names))
            .expect("Profile was created incorrectly, revise test example");

        let result = SimplePluralityRule::<30>.execute(&profile);

        assert!(matches!(
            result,
            Err(QParetianError::CombinatorialExplosion {
                limit: _,
                supplied: _
            })
        ));
    }
}
