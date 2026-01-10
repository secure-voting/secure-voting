//! Copeland I module.
//!
//! This module defines the Copeland I scorer.

use std::convert::Infallible;

use crate::{
    matrix::PairwiseMatrix,
    prelude::Profile,
    scorer::{Score, Scorer},
};

/// Copeland I scorer.
///
/// The score of each candidate is equal to the
/// number of wins in each head-to-head.
#[derive(Debug, Clone, Copy)]
pub struct CopelandIScorer;

impl Scorer for CopelandIScorer {
    type Output = Vec<usize>;

    type Error = Infallible;

    fn compute_score(&self, profile: &Profile) -> Result<Score<Self::Output>, Self::Error> {
        let pairwise = PairwiseMatrix::from(profile);
        let n_candidates = profile.n_candidates();

        let scores = (0..n_candidates)
            .map(|i| {
                (0..n_candidates)
                    .filter(|&j| i != j && pairwise.beats(i, j))
                    .count()
            })
            .collect();

        Ok(Score::new(scores, profile.active_candidates()))
    }

    fn new() -> Self {
        Self
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn copeland_single_vote_linear_order() {
        let votes = vec![vec![0, 1, 2, 3]];
        let profile = votes.try_into().unwrap();

        let scores = CopelandIScorer.compute_score(&profile).unwrap();

        assert_eq!(scores.score().clone(), vec![3, 2, 1, 0]);
    }

    #[test]
    fn copeland_condorcet_winner() {
        let votes = vec![vec![0, 1, 2], vec![0, 2, 1], vec![0, 1, 2]];
        let profile = votes.try_into().unwrap();

        let scores = CopelandIScorer.compute_score(&profile).unwrap();

        assert_eq!(scores.score().clone(), vec![2, 1, 0]);
    }

    #[test]
    fn copeland_cycle() {
        let votes = vec![vec![0, 1, 2], vec![1, 2, 0], vec![2, 0, 1]];
        let profile = votes.try_into().unwrap();

        let scores = CopelandIScorer.compute_score(&profile).unwrap();

        assert_eq!(scores.score().clone(), vec![1, 1, 1]);
    }

    #[test]
    fn copeland_two_candidates() {
        let votes = vec![vec![0, 1], vec![0, 1], vec![1, 0]];
        let profile = votes.try_into().unwrap();

        let scores = CopelandIScorer.compute_score(&profile).unwrap();

        assert_eq!(scores.score().clone(), vec![1, 0]);
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

        let scores = CopelandIScorer.compute_score(&profile).unwrap();

        assert_eq!(scores.score().clone(), vec![0, 0, 0]);
    }
}
