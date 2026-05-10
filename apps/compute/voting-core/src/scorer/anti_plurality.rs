//! Anti-plurality scorer implementation.
//!
//! Votes are counted for all candidates except the last one.

use std::convert::Infallible;

use rayon::prelude::*;

use crate::{
    models::{profile::Profile, ranking::RankingBallot},
    scorer::{Score, Scorer},
};

/// Anti-plurality scorer.
///
/// Gives one point to everyone except the bottom candidate.
#[derive(Debug, Clone, Copy)]
pub struct AntiPluralityScorer;

impl Scorer<RankingBallot> for AntiPluralityScorer {
    type Error = Infallible;
    type Output = Vec<usize>;

    fn compute_score(
        &self,
        profile: &Profile<RankingBallot>,
    ) -> Result<Score<Self::Output>, Self::Error> {
        let n_voters = profile.n_voters();
        let n_candidates = profile.n_candidates();

        // The unwrap is used on a get_candidate_id return value,
        // which is called with a profile-related value, so is safe.
        #[allow(clippy::unwrap_used)]
        Ok(Score::new(
            (0..n_voters)
                .into_par_iter()
                .map(|i| {
                    let mut tmp = vec![0usize; n_candidates];

                    tmp[profile.index_of(profile[i].iter().last().unwrap()).unwrap()] = 1;

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

    #[test_case(
    vec![
        vec![1, 0, 2],
        vec![0, 2, 1],
        vec![1, 2, 0]
    ],
    &[1, 1, 1];
    "case_1"
)]
    #[test_case(
    vec![
        vec![0, 1, 2, 3],
        vec![0, 1, 3, 2],
        vec![2, 3, 1, 0]
    ],
    &[1, 0, 1, 1];
    "case_2"
)]
    fn test_correct_anti_plurality(votes: Vec<Vec<usize>>, answer: &[usize]) {
        let scorer = AntiPluralityScorer;

        let n = votes[0].len();
        let names: Vec<String> = (0..n).map(|i| format!("C{i}")).collect::<Vec<_>>();
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
        let profile = Profile::try_from((ballots, names))
            .expect("Profile is constructed incorrectly, revise test examples.");

        assert_eq!(answer, scorer.compute_score(&profile).unwrap().score());
    }
}
