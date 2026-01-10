//! Approval scorer implementation.
//!
//! Scores the first Q candidates

use rayon::prelude::*;
use thiserror::Error;

use crate::{
    profile::Profile,
    scorer::{Score, Scorer},
};

/// Q-Approval scorer.
///
/// Gives one point to the first Q-candidates.
/// This type is a zero-sized marker implementing [`Scorer`].
pub struct ApprovalScorer<const Q: usize>;

/// Approval error type.
///
/// Approval scoring has only one way to fail: not enough candidates to score the first Q.
#[derive(Debug, Error)]
#[error("Not enough candidates for this Q")]
pub struct ApprovalScorerError;

impl<const Q: usize> Scorer for ApprovalScorer<Q> {
    type Output = Vec<usize>;
    type Error = ApprovalScorerError;

    fn compute_score(&self, profile: &Profile) -> Result<Score<Self::Output>, Self::Error> {
        let n_voters = profile.n_voters();
        let n_candidates = profile.n_candidates();

        if n_candidates < Q {
            return Err(ApprovalScorerError);
        }

        // The unwrap is used on a get_candidate_id return value,
        // which is called with a profile-related value, so is safe.
        #[allow(clippy::unwrap_used)]
        Ok(Score::new(
            (0..n_voters)
                .into_par_iter()
                .map(|i| {
                    let mut tmp = vec![0; n_candidates];

                    (0..Q).for_each(|x| tmp[profile.index_of(&profile[i][x]).unwrap()] = 1);

                    tmp
                })
                .reduce(
                    || vec![0; n_candidates],
                    |a, b| a.iter().zip(b.iter()).map(|(x, y)| x + y).collect(),
                ),
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
    use test_case::test_case;

    #[test_case(vec![vec![1, 0], vec![0, 1], vec![1, 0]], vec![3, 3]; "count all")]
    #[test_case(vec![vec![1, 0, 2], vec![0, 2, 1], vec![0, 2, 1]], vec![3, 1, 2]; "count top 2")]
    fn test_correct_approval_scoring(votes: Vec<Vec<usize>>, answer: Vec<usize>) {
        let scorer = ApprovalScorer::<2>;

        assert_eq!(
            answer,
            scorer
                .compute_score(&votes.try_into().unwrap())
                .unwrap()
                .score()
                .clone()
        );
    }

    #[test_case(vec![vec![0, 1], vec![1, 0], vec![0, 1]]; "less than q")]
    fn test_incorrect_too_little_candidates_for_this_q(votes: Vec<Vec<usize>>) {
        let scorer = ApprovalScorer::<3>;

        assert!(matches!(
            scorer.compute_score(&votes.try_into().unwrap()),
            Err(ApprovalScorerError)
        ));
    }
}
