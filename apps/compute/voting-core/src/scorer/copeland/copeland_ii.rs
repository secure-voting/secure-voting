//! Copeland II module.
//!
//! This module defines the Copeland II scorer.

use std::convert::Infallible;

use crate::{matrix::PairwiseMatrix, prelude::Profile, scorer::Scorer};

/// Copeland II scorer.
///
/// The score of each candidate is equal to the
/// difference in strict wins and strict losses.
pub struct CopelandIIScorer;

impl Scorer for CopelandIIScorer {
    type Output = Vec<isize>;

    type Error = Infallible;

    fn compute_score(&self, profile: &Profile) -> Result<Self::Output, Self::Error> {
        let pairwise = PairwiseMatrix::from(profile);
        let n_candidates = profile.n_candidates();

        let scores = (0..n_candidates)
            .map(|i| {
                (0..n_candidates)
                    .map(|j| {
                        if pairwise.beats(i, j) {
                            1
                        } else if pairwise.beats(j, i) {
                            -1
                        } else {
                            0
                        }
                    })
                    .sum()
            })
            .collect();

        Ok(scores)
    }
}
