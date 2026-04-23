//! Copeland II module.
//!
//! This module defines the Copeland II scorer.

use std::convert::Infallible;

use crate::{
    matrix::PairwiseMatrix,
    models::ranking::RankingBallot,
    prelude::Profile,
    scorer::{Score, Scorer},
};

/// Copeland II scorer.
///
/// The score of each candidate is equal to the
/// difference in strict wins and strict losses
/// in each head-to-head.
///
/// Ties contribute 0.
#[derive(Debug, Clone, Copy)]
pub struct CopelandIIScorer;

impl Scorer<RankingBallot> for CopelandIIScorer {
    type Output = Vec<isize>;

    type Error = Infallible;

    fn compute_score(
        &self,
        profile: &Profile<RankingBallot>,
    ) -> Result<Score<Self::Output>, Self::Error> {
        let pairwise = PairwiseMatrix::from(profile);
        let n_candidates = profile.n_candidates();

        let scores = (0..n_candidates)
            .map(|i| {
                (0..n_candidates)
                    .filter(|&j| i != j)
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

        Ok(Score::new(scores, profile.active_candidates()))
    }

    fn new() -> Self {
        Self
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn profile(votes: Vec<Vec<usize>>, n: usize) -> Profile<RankingBallot> {
        let names: Vec<String> = (0..n).map(|i| format!("C{i}")).collect();

        Profile::try_from((votes, names))
            .expect("Profile is constructed incorrectly, revise test example.")
    }

    #[test]
    fn copeland_single_vote_linear_order() {
        let profile = profile(vec![vec![0, 1, 2, 3]], 4);

        let scores = CopelandIIScorer.compute_score(&profile).unwrap();

        assert_eq!(scores.consume_score(), vec![3, 1, -1, -3]);
    }

    #[test]
    fn copeland_condorcet_winner() {
        let profile = profile(vec![vec![0, 1, 2], vec![0, 2, 1], vec![0, 1, 2]], 3);

        let scores = CopelandIIScorer.compute_score(&profile).unwrap();

        assert_eq!(scores.consume_score(), vec![2, 0, -2]);
    }

    #[test]
    fn copeland_cycle() {
        let profile = profile(vec![vec![0, 1, 2], vec![1, 2, 0], vec![2, 0, 1]], 3);

        let scores = CopelandIIScorer.compute_score(&profile).unwrap();

        assert_eq!(scores.consume_score(), vec![0, 0, 0]);
    }

    #[test]
    fn copeland_two_candidates() {
        let profile = profile(vec![vec![0, 1], vec![0, 1], vec![1, 0]], 2);

        let scores = CopelandIIScorer.compute_score(&profile).unwrap();

        assert_eq!(scores.consume_score(), vec![1, -1]);
    }

    #[test]
    fn copeland_all_tied() {
        let profile = profile(
            vec![
                vec![0, 1, 2],
                vec![1, 2, 0],
                vec![2, 0, 1],
                vec![0, 2, 1],
                vec![1, 0, 2],
                vec![2, 1, 0],
            ],
            3,
        );

        let scores = CopelandIIScorer.compute_score(&profile).unwrap();

        assert_eq!(scores.consume_score(), vec![0, 0, 0]);
    }
}
