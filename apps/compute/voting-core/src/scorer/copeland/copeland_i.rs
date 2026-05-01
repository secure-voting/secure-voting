//! Copeland I module.
//!
//! This module defines the Copeland I scorer.

use std::convert::Infallible;

use crate::{
    matrix::PairwiseMatrix,
    models::ranking::RankingBallot,
    prelude::Profile,
    scorer::{Score, Scorer},
};

/// Copeland I scorer.
///
/// The score of each candidate is equal to the
/// number of wins in each head-to-head.
#[derive(Debug, Clone, Copy)]
pub struct CopelandIScorer;

impl Scorer<RankingBallot> for CopelandIScorer {
    type Output = Vec<usize>;

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
    use crate::models::candidate_id::CandidateId;
    use crate::models::BallotData;

    fn profile(votes: Vec<Vec<usize>>, n: usize) -> Profile<RankingBallot> {
        let names: Vec<String> = (0..n).map(|i| format!("C{i}")).collect();
        let ballots: Vec<BallotData> = votes
            .into_iter()
            .map(|v| {
                BallotData::Simple(
                    v.into_iter()
                        .map(|id| CandidateId::new(id, format!("C{id}")))
                        .collect(),
                )
            })
            .collect();

        Profile::try_from((ballots, names))
            .expect("Profile is constructed incorrectly, revise test example.")
    }

    #[test]
    fn copeland_single_vote_linear_order() {
        let profile = profile(vec![vec![0, 1, 2, 3]], 4);

        let scores = CopelandIScorer.compute_score(&profile).unwrap();

        assert_eq!(scores.consume_score(), vec![3, 2, 1, 0]);
    }

    #[test]
    fn copeland_condorcet_winner() {
        let profile = profile(vec![vec![0, 1, 2], vec![0, 2, 1], vec![0, 1, 2]], 3);

        let scores = CopelandIScorer.compute_score(&profile).unwrap();

        assert_eq!(scores.consume_score(), vec![2usize, 1, 0]);
    }

    #[test]
    fn copeland_cycle() {
        let profile = profile(vec![vec![0, 1, 2], vec![1, 2, 0], vec![2, 0, 1]], 3);

        let scores = CopelandIScorer.compute_score(&profile).unwrap();

        assert_eq!(scores.consume_score(), vec![1usize, 1, 1]);
    }

    #[test]
    fn copeland_two_candidates() {
        let profile = profile(vec![vec![0, 1], vec![0, 1], vec![1, 0]], 2);

        let scores = CopelandIScorer.compute_score(&profile).unwrap();

        assert_eq!(scores.consume_score(), vec![1usize, 0]);
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

        let scores = CopelandIScorer.compute_score(&profile).unwrap();

        assert_eq!(scores.consume_score(), vec![0usize, 0, 0]);
    }
}
