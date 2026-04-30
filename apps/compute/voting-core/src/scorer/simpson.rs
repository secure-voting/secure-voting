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
    type Output = Vec<isize>;

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
    use crate::models::BallotData;

    fn profile(votes: Vec<Vec<usize>>, n: usize) -> Profile<RankingBallot> {
        let names: Vec<String> = (0..n).map(|i| format!("C{i}")).collect();
        let ballots: Vec<BallotData> = votes.into_iter().map(BallotData::Simple).collect();

        Profile::try_from((ballots, names))
            .expect("Profile is constructed incorrectly, revise test example.")
    }
    #[test]
    fn simpson_single_vote_linear_order() {
        let profile = profile(vec![vec![0, 1, 2, 3]], 4);

        let scores = SimpsonScorer.compute_score(&profile).unwrap();

        assert_eq!(scores.consume_score(), vec![1, -1, -1, -1]);
    }
    #[test]
    fn simpson_condorcet_winner() {
        let profile = profile(vec![vec![0, 1, 2], vec![0, 2, 1], vec![0, 1, 2]], 3);

        let scores = SimpsonScorer.compute_score(&profile).unwrap();

        assert_eq!(scores.consume_score(), vec![3, -3, -3]);
    }
    #[test]
    fn simpson_condorcet_cycle() {
        let profile = profile(vec![vec![0, 1, 2], vec![1, 2, 0], vec![2, 0, 1]], 3);

        let scores = SimpsonScorer.compute_score(&profile).unwrap();

        assert_eq!(scores.consume_score(), vec![-1, -1, -1]);
    }
    #[test]
    fn simpson_two_candidates() {
        let profile = profile(vec![vec![0, 1], vec![0, 1], vec![1, 0]], 2);

        let scores = SimpsonScorer.compute_score(&profile).unwrap();

        assert_eq!(scores.consume_score(), vec![1, -1]);
    }
    #[test]
    fn simpson_all_pairwise_tied() {
        let profile = profile(
            vec![
                vec![0, 1, 2],
                vec![0, 2, 1],
                vec![1, 0, 2],
                vec![1, 2, 0],
                vec![2, 0, 1],
                vec![2, 1, 0],
            ],
            3,
        );

        let scores = SimpsonScorer.compute_score(&profile).unwrap();

        assert_eq!(scores.consume_score(), vec![0, 0, 0]);
    }
    #[test]
    fn simpson_worst_loss_dominates() {
        let profile = profile(
            vec![
                vec![0, 1, 2],
                vec![0, 1, 2],
                vec![0, 1, 2],
                vec![2, 0, 1],
                vec![2, 0, 1],
            ],
            3,
        );

        let scores = SimpsonScorer.compute_score(&profile).unwrap();

        assert_eq!(scores.consume_score(), vec![1, -5, -1]);
    }
}
