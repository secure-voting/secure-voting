//! Scoring scorer implementation.
//!
//! Scores of each candidate are summed up.

use std::convert::Infallible;

use rayon::prelude::*;

use crate::{
    models::{profile::Profile, scoring::ScoreBallot},
    scorer::{Score, Scorer},
};

/// Scoring scorer.
///
/// Each candidate gets a score from the ballot, the scores are then summed per candidate.
#[derive(Debug, Clone, Copy)]
pub struct ScoringScorer;

impl Scorer<ScoreBallot> for ScoringScorer {
    type Error = Infallible;
    type Output = Vec<usize>;

    fn compute_score(
        &self,
        profile: &Profile<ScoreBallot>,
    ) -> Result<Score<Self::Output>, Self::Error> {
        let n_voters = profile.n_voters();
        let active_candidates = profile.active_candidates();

        #[allow(clippy::cast_possible_truncation)]
        Ok(Score::new(
            (0..n_voters)
                .into_par_iter()
                .map(|i| {
                    let ballot = &profile[i];
                    active_candidates
                        .iter()
                        .map(|c| ballot.get_score(c).unwrap_or(0))
                        .collect::<Vec<_>>()
                })
                .reduce(
                    || vec![0; active_candidates.len()],
                    |a, b| a.iter().zip(b.iter()).map(|(x, y)| x + y).collect(),
                ),
            active_candidates,
        ))
    }

    fn new() -> Self {
        Self
    }
}

#[allow(clippy::unwrap_used)]
#[cfg(test)]
mod tests {
    use super::*;
    use crate::models::BallotData;
    use crate::models::candidate_id::CandidateId;
    use test_case::test_case;

    fn make_scoring_ballots(votes: Vec<Vec<(usize, usize)>>, names: &[String]) -> Vec<BallotData> {
        votes
            .into_iter()
            .map(|v| {
                BallotData::Scoring(
                    v.into_iter()
                        .map(|(id, score)| (CandidateId::new(id, names[id].clone()), score))
                        .collect(),
                )
            })
            .collect()
    }

    #[test_case(
        vec![
            vec![(0, 3), (1, 2), (2, 1)],
            vec![(0, 3), (1, 2), (2, 1)],
            vec![(1, 3), (2, 2), (0, 1)],
        ],
        &[7, 7, 4];
        "simple scoring"
    )]
    #[test_case(
        vec![
            vec![(0, 5), (1, 5)],
            vec![(0, 3), (1, 5)],
        ],
        &[8, 10];
        "two candidates equal scores"
    )]
    #[test_case(
        vec![
            vec![(0, 1)],
            vec![(1, 1)],
            vec![(2, 1)],
        ],
        &[1, 1, 1];
        "partial ballots one point each"
    )]
    fn test_correct_scoring(votes: Vec<Vec<(usize, usize)>>, answer: &[usize]) {
        let names: Vec<String> = (0..3).map(|i| format!("C{i}")).collect::<Vec<_>>();
        let ballots = make_scoring_ballots(votes, &names);

        let profile = Profile::try_from((ballots, names))
            .expect("Profile is constructed incorrectly, revise test example.");

        assert_eq!(
            answer,
            ScoringScorer
                .compute_score(&profile)
                .unwrap()
                .consume_score()
        );
    }
}
