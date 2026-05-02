//! Condorcet scorer implementation.
//!
//! Calculates all head-to-head candidates' scores.

use std::convert::Infallible;

use crate::{
    matrix::{CondorcetMatrix, PairwiseMatrix},
    models::{profile::Profile, ranking::RankingBallot},
    scorer::{Score, Scorer},
};

/// Condorcet Scorer type.
///
/// Gives points only of the candidate wins in a head-to-head.
#[derive(Debug, Clone, Copy)]
pub struct CondorcetScorer;

impl Scorer<RankingBallot> for CondorcetScorer {
    /// Matrix of head-to-head preferences.
    ///
    /// Each row describes a candidate.
    /// If there is a zero on the i-th position, then current candidate loses to i-th in a head-to-head.
    /// If there is a one on the i-th position, then current candidate wins to i-th in a head-to-head.
    type Output = CondorcetMatrix;

    type Error = Infallible;

    fn compute_score(
        &self,
        profile: &Profile<RankingBallot>,
    ) -> Result<Score<Self::Output>, Self::Error> {
        let vote_counts = PairwiseMatrix::from(profile);
        // The construction process ensures the invariants of the matrix are upheld
        Ok(Score::new(
            CondorcetMatrix::from(vote_counts),
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
    use crate::models::BallotData;
    use crate::models::candidate_id::CandidateId;

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
    fn correct_condorcet_matrix_single_vote() {
        let votes = vec![vec![1, 2, 0, 3]];

        let answer = vec![
            vec![false, false, false, true],
            vec![true, false, true, true],
            vec![true, false, false, true],
            vec![false, false, false, false],
        ];

        let profile = profile(votes, 4);

        let result = CondorcetScorer
            .compute_score(&profile)
            .unwrap()
            .score()
            .clone();

        assert_eq!(answer, Vec::<Vec<bool>>::from(result));
    }

    #[test]
    fn correct_condorcet_matrix_multiple_votes() {
        let votes = vec![vec![1, 2, 0, 3], vec![3, 0, 2, 1], vec![0, 2, 1, 3]];

        let answer = vec![
            vec![false, true, true, true],
            vec![false, false, false, true],
            vec![false, true, false, true],
            vec![false, false, false, false],
        ];

        let profile = profile(votes, 4);

        let result = CondorcetScorer
            .compute_score(&profile)
            .unwrap()
            .score()
            .clone();

        assert_eq!(answer, Vec::<Vec<bool>>::from(result));
    }
}
