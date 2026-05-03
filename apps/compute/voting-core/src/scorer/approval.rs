//! Approval scorer implementation.
//!
//! Scores the first Q candidates

use rayon::prelude::*;
use thiserror::Error;

use crate::{
    models::{approval::ApprovalBallot, profile::Profile, ranking::RankingBallot},
    scorer::{Score, Scorer},
};

/// Q-Approval scorer.
///
/// Gives one point to the first Q-candidates.
/// This type is a zero-sized marker implementing [`Scorer`].
#[derive(Debug, Clone, Copy)]
pub struct ApprovalScorer<const Q: usize>;

/// Approval error type.
///
/// Approval scoring has only one way to fail: wrong count of candidates in a ballot for this Q.
#[derive(Debug, Error)]
#[error("Not enough candidates for this Q")]
pub struct ApprovalScorerError;

impl<const Q: usize> Scorer<RankingBallot> for ApprovalScorer<Q> {
    type Output = Vec<usize>;
    type Error = ApprovalScorerError;

    fn compute_score(
        &self,
        profile: &Profile<RankingBallot>,
    ) -> Result<Score<Self::Output>, Self::Error> {
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

impl<const Q: usize> Scorer<ApprovalBallot> for ApprovalScorer<Q> {
    type Output = Vec<usize>;
    type Error = ApprovalScorerError;

    fn compute_score(
        &self,
        profile: &Profile<ApprovalBallot>,
    ) -> Result<Score<Self::Output>, Self::Error> {
        let n_voters = profile.n_voters();
        let n_candidates = profile.n_candidates();

        if (0..n_voters).any(|i| profile[i].iter().count() > Q) {
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

                    for candidate in profile[i].iter() {
                        tmp[profile.index_of(candidate).unwrap()] = 1;
                    }

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
    use crate::models::BallotData;
    use crate::models::candidate_id::CandidateId;
    use test_case::test_case;

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
            .expect("Profile is constructed incorrectly, revise test examples.")
    }

    #[test_case(
    vec![vec![1, 0], vec![0, 1], vec![1, 0]],
    &[3, 3];
    "count all"
)]
    #[test_case(
    vec![vec![1, 0, 2], vec![0, 2, 1], vec![0, 2, 1]],
    &[3, 1, 2];
    "count top 2"
)]
    fn test_correct_approval_scoring(votes: Vec<Vec<usize>>, answer: &[usize]) {
        let scorer = ApprovalScorer::<2>;

        let profile = profile(votes, 3);

        assert_eq!(
            answer,
            scorer
                .compute_score(&profile)
                .expect("Scorer should not fail if Q is bigger than candidate count")
                .score()
        );
    }

    #[test_case(
    vec![vec![0, 1], vec![1, 0], vec![0, 1]];
    "less than q"
)]
    fn test_incorrect_too_little_candidates_for_this_q(votes: Vec<Vec<usize>>) {
        let scorer = ApprovalScorer::<3>;

        let profile = profile(votes, 2);

        assert!(matches!(
            scorer.compute_score(&profile),
            Err(ApprovalScorerError)
        ));
    }
}
