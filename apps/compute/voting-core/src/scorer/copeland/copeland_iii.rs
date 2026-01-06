//! Copeland III module.
//!
//! This module defines the Copeland III scorer.

use std::convert::Infallible;

use crate::{matrix::PairwiseMatrix, prelude::Profile, scorer::Scorer};

/// Copeland III scorer.
///
/// The score of each candidate is equal to the
/// total margin between the chosen candidates and alternatives.
pub struct CopelandIIIScorer;

impl Scorer for CopelandIIIScorer {
    type Output = Vec<isize>;

    type Error = Infallible;

    fn compute_score(&self, profile: &Profile) -> Result<Self::Output, Self::Error> {
        let pairwise = PairwiseMatrix::from(profile);
        let n_candidates = profile.n_candidates();

        let scores = (0..n_candidates)
            .map(|i| {
                (0..n_candidates)
                    .filter(|&j| j != i)
                    .map(|j| pairwise.margin(i, j))
                    .sum()
            })
            .collect();

        Ok(scores)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn copeland_single_vote_linear_order() {
        let votes = vec![vec![0, 1, 2, 3]];
        let profile = votes.try_into().unwrap();

        let scores = CopelandIIIScorer.compute_score(&profile).unwrap();

        assert_eq!(scores, vec![3, 1, -1, -3]);
    }

    #[test]
    fn copeland_condorcet_winner() {
        let votes = vec![vec![0, 1, 2], vec![0, 2, 1], vec![0, 1, 2]];
        let profile = votes.try_into().unwrap();

        let scores = CopelandIIIScorer.compute_score(&profile).unwrap();

        assert_eq!(scores, vec![6, -2, -4]);
    }

    #[test]
    fn copeland_cycle() {
        let votes = vec![vec![0, 1, 2], vec![1, 2, 0], vec![2, 0, 1]];
        let profile = votes.try_into().unwrap();

        let scores = CopelandIIIScorer.compute_score(&profile).unwrap();

        assert_eq!(scores, vec![0, 0, 0]);
    }

    #[test]
    fn copeland_two_candidates() {
        let votes = vec![vec![0, 1], vec![0, 1], vec![1, 0]];
        let profile = votes.try_into().unwrap();

        let scores = CopelandIIIScorer.compute_score(&profile).unwrap();

        assert_eq!(scores, vec![1, -1]);
    }

    #[test]
    fn copeland_all_tied() {
        let votes = vec![
            vec![0, 1, 2],
            vec![1, 2, 0],
            vec![2, 0, 1],
            vec![0, 2, 1],
            vec![1, 0, 2],
            vec![2, 1, 0],
        ];
        let profile = votes.try_into().unwrap();

        let scores = CopelandIIIScorer.compute_score(&profile).unwrap();

        assert_eq!(scores, vec![0, 0, 0]);
    }
}
