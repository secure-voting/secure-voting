//! Threshold scorer implementation.
//!
//! Score of the candidate is a vector of values `v_i(x)` representing the
//! number of `i`-th places `x` got in the profile.

use std::{convert::Infallible, marker::PhantomData};

use rayon::prelude::*;

use crate::{
    models::{profile::Profile, ranking::RankingBallot},
    scorer::{Score, Scorer},
};

/// Threshold scorer.
///
/// Counts how many times each candidate was on each place in the ballots of the profile.
#[derive(Debug, Clone, Copy)]
pub struct ThresholdScorer<Ballot> {
    /// Ballot type marker.
    _ballot_type: PhantomData<Ballot>,
}

impl Scorer<RankingBallot> for ThresholdScorer<RankingBallot> {
    type Output = Vec<Vec<usize>>;

    type Error = Infallible;

    fn compute_score(
        &self,
        profile: &Profile<RankingBallot>,
    ) -> Result<Score<Self::Output>, Self::Error> {
        let n_voters = profile.n_voters();
        let n_candidates = profile.n_candidates();

        #[allow(clippy::unwrap_used)]
        Ok(Score::new(
            (0..n_voters)
                .into_par_iter()
                .map(|i| {
                    let mut tmp = vec![vec![0; n_candidates - 1]; n_candidates];

                    for j in 0..(n_candidates - 1) {
                        let candidate = &profile[i][j];
                        let c_idx = profile.index_of(candidate).unwrap();

                        tmp[c_idx][j] += 1;
                    }

                    tmp
                })
                .reduce(
                    || vec![vec![0; n_candidates - 1]; n_candidates],
                    |a, b| {
                        a.iter()
                            .zip(b.iter())
                            .map(|(va, vb)| va.iter().zip(vb.iter()).map(|(x, y)| x + y).collect())
                            .collect()
                    },
                ),
            profile.active_candidates(),
        ))
    }

    fn new() -> Self {
        Self {
            _ballot_type: PhantomData,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use test_case::test_case;

    #[test_case(vec![vec![0, 1, 2, 3], vec![0, 1, 2, 3], vec![1, 2, 3, 0]], &[vec![2, 0, 0], vec![1, 2, 0], vec![0, 1, 2], vec![0, 0, 1]]; "simple example")]
    fn correct_threshold_ranking(votes: Vec<Vec<usize>>, answer: &[Vec<usize>]) {
        assert_eq!(
            answer,
            ThresholdScorer::<RankingBallot>::new()
                .compute_score(
                    &votes
                        .try_into()
                        .expect("Profile is constructed incorrectly, revise test examples.")
                )
                .unwrap()
                .score()
                .clone()
        );
    }
}
