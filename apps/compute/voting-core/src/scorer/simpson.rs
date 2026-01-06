//! Simpson scorer module.

use std::convert::Infallible;

use crate::{matrix::PairwiseMatrix, prelude::Profile, scorer::Scorer};

/// Simpson scorer type.
///
/// Score of candidate `i` is equal to
/// minimum pairwise victory margin against any opponent.
#[derive(Clone, Copy, Debug)]
pub struct SimpsonScorer;

impl Scorer for SimpsonScorer {
    type Output = Vec<isize>;

    type Error = Infallible;

    fn compute_score(&self, profile: &Profile) -> Result<Self::Output, Self::Error> {
        let pairwise_matrix = PairwiseMatrix::from(profile);
        let n = pairwise_matrix.n();

        // unwrap is justified because the pairwise matrix
        // can't be empty per its invariants
        #[allow(clippy::unwrap_used)]
        Ok((0..n)
            .map(|i| {
                (0..n)
                    .filter(|j| *j != i)
                    .map(|j| pairwise_matrix.margin(i, j))
                    .min()
                    .unwrap()
            })
            .collect())
    }
}
