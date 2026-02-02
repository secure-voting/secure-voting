//! Minmax scorer module.

use std::convert::Infallible;

use crate::{
    matrix::PairwiseMatrix,
    models::ranking::RankingBallot,
    prelude::Profile,
    scorer::{Score, Scorer},
};

/// Minmax scorer type.
///
/// Score of candidate `i` is equal to
/// the maximum pairwise loss margin against any opponent.
#[derive(Clone, Copy, Debug)]
pub struct MinmaxScorer;

impl Scorer<RankingBallot> for MinmaxScorer {
    type Output = Vec<isize>;
    type Error = Infallible;

    fn compute_score(
        &self,
        profile: &Profile<RankingBallot>,
    ) -> Result<Score<Self::Output>, Self::Error> {
        let pairwise_matrix = PairwiseMatrix::from(profile);
        let n = pairwise_matrix.n();

        #[allow(clippy::unwrap_used)]
        Ok(Score::new(
            (0..n)
                .map(|i| {
                    (0..n)
                        .filter(|&j| j != i)
                        .map(|j| pairwise_matrix.margin(j, i))
                        .max()
                        .unwrap()
                })
                .collect(),
            profile.active_candidates(),
        ))
    }

    fn new() -> Self {
        Self
    }
}
