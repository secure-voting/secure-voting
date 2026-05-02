//! Plurality scorer implementation.
//!
//! Votes are counted only for the first candidate.

use std::convert::Infallible;

use rayon::prelude::*;

use crate::{
    models::{profile::Profile, ranking::RankingBallot},
    scorer::{Score, Scorer},
};

/// Plurality scorer.
///
/// Gives one point to the top candidate.
#[derive(Debug, Clone, Copy)]
pub struct PluralityScorer;

impl Scorer<RankingBallot> for PluralityScorer {
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
                    tmp[profile.index_of(&profile[i][0]).unwrap()] = 1;

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
    vec![vec![1, 0], vec![0, 1], vec![1, 0]],
    &[1, 2];
    "simple plurality"
)]
    fn test_correct_simple_plurality(votes: Vec<Vec<usize>>, answer: &[usize]) {
        let scorer = PluralityScorer;

        let names: Vec<String> = vec!["A".into(), "B".into()];
        let ballots: Vec<BallotData> = votes
            .into_iter()
            .map(|v| {
                let names_ref = names.clone();
                BallotData::Simple(
                    v.into_iter()
                        .map(|id| CandidateId::new(id, names_ref[id].clone()))
                        .collect(),
                )
            })
            .collect();

        let profile = Profile::try_from((ballots, names))
            .expect("Profile is constructed incorrectly, revise test example.");

        assert_eq!(
            answer,
            scorer.compute_score(&profile).unwrap().consume_score()
        );
    }
}
