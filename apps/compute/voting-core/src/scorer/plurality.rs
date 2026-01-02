use rayon::prelude::*;
use thiserror::Error;

use crate::{profile::Profile, scorer::Scorer};

pub struct PluralityScorer;

#[derive(Debug, Error)]
#[error("Empty ballot")]
pub struct PluralityScorerError;

impl Scorer for PluralityScorer {
    type Error = PluralityScorerError;
    type Output = Vec<usize>;

    fn compute_score(&self, profile: &Profile) -> Result<Self::Output, Self::Error> {
        let n_voters = profile.n_voters();
        let n_candidates = profile.n_candidates();

        if n_candidates == 0 {
            return Err(PluralityScorerError);
        }

        Ok((0..n_voters)
            .into_par_iter()
            .map(|i| {
                let mut tmp = vec![0; n_candidates];
                if !tmp.is_empty() {
                    tmp[profile[i][0]] = 1;
                }

                tmp
            })
            .reduce(
                || vec![0; n_candidates],
                |a, b| a.iter().zip(b.iter()).map(|(x, y)| x + y).collect(),
            ))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use test_case::test_case;

    #[test_case(vec![vec![1, 0], vec![0, 1], vec![1, 0]], vec![1, 2]; "simple plurality")]
    fn test_correct_simple_plurality(votes: Vec<Vec<usize>>, answer: Vec<usize>) {
        let scorer = PluralityScorer;

        assert_eq!(
            answer,
            scorer.compute_score(&votes.try_into().unwrap()).unwrap()
        );
    }

    #[test]
    fn test_incorrect_empty_ballots() {
        let votes = vec![vec![]];
        let scorer = PluralityScorer;

        assert!(matches!(
            scorer.compute_score(&votes.try_into().unwrap()),
            Err(PluralityScorerError)
        ));
    }
}
