//! Copeland III module.
//!
//! This module defines the Copeland III scorer.

use std::convert::Infallible;

use crate::{matrix::PairwiseMatrix, prelude::Profile, scorer::Scorer};

/// Copeland III scorer.
///
/// The score of each candidate is equal to the
/// number of wins in each head-to-head.
pub struct CopelandIIIScorer;

impl Scorer for CopelandIIIScorer {
    type Output = Vec<isize>;

    type Error = Infallible;

    fn compute_score(&self, profile: &Profile) -> Result<Self::Output, Self::Error> {
        let pairwise = PairwiseMatrix::from(profile);
        let n_candidates = profile.n_candidates();

        let scores = (0..n_candidates)
            .map(|i| (0..n_candidates).map(|j| pairwise.margin(i, j)).sum())
            .collect();

        Ok(scores)
    }
}
