//! Simpson scorer module.

use std::convert::Infallible;

use crate::{
    matrix::PairwiseMatrix,
    models::ranking::RankingBallot,
    prelude::Profile,
    scorer::{Score, Scorer},
};

/// Simpson scorer type.
///
/// Score of candidate `i` is equal to
/// minimum pairwise victory margin against any opponent.
#[derive(Clone, Copy, Debug)]
pub struct SimpsonScorer;

impl Scorer<RankingBallot> for SimpsonScorer {
    type Output = isize;

    type Error = Infallible;

    fn compute_score(
        &self,
        profile: &Profile<RankingBallot>,
    ) -> Result<Score<Self::Output>, Self::Error> {
        let pairwise_matrix = PairwiseMatrix::from(profile);
        let n = pairwise_matrix.n();

        // unwrap is justified because the pairwise matrix
        // can't be empty per its invariants
        #[allow(clippy::unwrap_used)]
        Ok(Score::new(
            (0..n)
                .map(|i| {
                    (0..n)
                        .filter(|j| *j != i)
                        .map(|j| pairwise_matrix.margin(i, j))
                        .min()
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

#[cfg(test)]
mod tests {
    use super::*;
    use crate::models::profile::Profile;

    #[test]
    fn simpson_single_vote_linear_order() {
        let profile = Profile::try_from(vec![vec![0, 1, 2, 3]])
            .expect("Profile is constructed incorrectly, revise test example.");

        let scores = SimpsonScorer.compute_score(&profile).unwrap();

        assert_eq!(scores.score().clone(), vec![1, -1, -1, -1]);
    }

    #[test]
    fn simpson_condorcet_winner() {
        let profile = Profile::try_from(vec![vec![0, 1, 2], vec![0, 2, 1], vec![0, 1, 2]])
            .expect("Profile is constructed incorrectly, revise test example.");

        let scores = SimpsonScorer.compute_score(&profile).unwrap();

        assert_eq!(scores.score().clone(), vec![3, -3, -3]);
    }

    #[test]
    fn simpson_condorcet_cycle() {
        let profile = Profile::try_from(vec![vec![0, 1, 2], vec![1, 2, 0], vec![2, 0, 1]])
            .expect("Profile is constructed incorrectly, revise test example.");

        let scores = SimpsonScorer.compute_score(&profile).unwrap();

        assert_eq!(scores.score().clone(), vec![-1, -1, -1]);
    }

    #[test]
    fn simpson_two_candidates() {
        let profile = Profile::try_from(vec![vec![0, 1], vec![0, 1], vec![1, 0]])
            .expect("Profile is constructed incorrectly, revise test example.");

        let scores = SimpsonScorer.compute_score(&profile).unwrap();

        assert_eq!(scores.score().clone(), vec![1, -1]);
    }

    #[test]
    fn simpson_all_pairwise_tied() {
        let profile = Profile::try_from(vec![
            vec![0, 1, 2],
            vec![0, 2, 1],
            vec![1, 0, 2],
            vec![1, 2, 0],
            vec![2, 0, 1],
            vec![2, 1, 0],
        ])
        .expect("Profile is constructed incorrectly, revise test example.");

        let scores = SimpsonScorer.compute_score(&profile).unwrap();

        assert_eq!(scores.score().clone(), vec![0, 0, 0]);
    }

    #[test]
    fn simpson_worst_loss_dominates() {
        // Candidate 0 wins big against 1, but loses badly to 2
        let profile = Profile::try_from(vec![
            vec![0, 1, 2],
            vec![0, 1, 2],
            vec![0, 1, 2],
            vec![2, 0, 1],
            vec![2, 0, 1],
        ])
        .expect("Profile is constructed incorrectly, revise test example.");

        let scores = SimpsonScorer.compute_score(&profile).unwrap();

        assert_eq!(scores.score().clone(), vec![1, -5, -1]);
    }
}
