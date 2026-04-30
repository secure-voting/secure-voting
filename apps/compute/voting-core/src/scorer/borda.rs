//! Borda scorer implementation.
//!
//! Scores each candidate depending on its rank. The last one gets 0 points, the next one gets one more and so on.

use std::{convert::Infallible, marker::PhantomData};

use rayon::prelude::*;

use crate::{
    models::{profile::Profile, ranking::RankingBallot},
    scorer::{Score, Scorer},
};

/// Borda scorer.
///
/// Gives n-1 points to the first candiate, n-2 to the second, ..., 1 to (n-1)th and 0 to the last.
#[derive(Debug, Clone, Copy)]
pub struct BordaScorer<Ballot> {
    /// Ballot type marker.
    _ballot_type: PhantomData<Ballot>,
}

impl Scorer<RankingBallot> for BordaScorer<RankingBallot> {
    type Output = Vec<usize>;

    type Error = Infallible;

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
                    let mut tmp = vec![0; n_candidates];

                    for j in 0..n_candidates {
                        tmp[profile.index_of(&profile[i][j]).unwrap()] = n_candidates - j - 1;
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
        Self {
            _ballot_type: PhantomData,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::models::BallotData;
    use test_case::test_case;

    #[test_case(
    vec![
        vec![0, 1, 2, 3],
        vec![0, 1, 2, 3],
        vec![1, 2, 3, 0]
    ],
    &[6, 7, 4, 1];
    "simple example"
)]
    fn test_correct_borda_ranking(votes: Vec<Vec<usize>>, answer: &[usize]) {
        let names = vec!["A".into(), "B".into(), "C".into(), "D".into()];
        let ballots: Vec<BallotData> = votes.into_iter().map(BallotData::Simple).collect();

        let profile = Profile::try_from((ballots, names))
            .expect("Profile is constructed incorrectly, revise test examples.");

        assert_eq!(
            answer,
            BordaScorer::<RankingBallot>::new()
                .compute_score(&profile)
                .unwrap()
                .score()
        );
    }
}
