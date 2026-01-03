//! Anti-plurality scorer implementation.
//!
//! Votes are counted for all candidates except the last one.

use std::convert::Infallible;

use rayon::prelude::*;

use crate::{profile::Profile, scorer::Scorer};

/// Anti-plurality scorer.
///
/// Gives one point to everyone except the bottom candidate.
pub struct AntiPluralityScorer;

impl Scorer for AntiPluralityScorer {
    type Error = Infallible;
    type Output = Vec<usize>;

    fn compute_score(&self, profile: &Profile) -> Result<Self::Output, Self::Error> {
        let n_voters = profile.n_voters();
        let n_candidates = profile.n_candidates();

        Ok((0..n_voters)
            .into_par_iter()
            .map(|i| {
                let mut tmp = vec![0; n_candidates];

                for j in 0..n_candidates - 1 {
                    tmp[profile[i][j].into_inner()] += 1;
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

    #[test_case(vec![vec![1, 0, 2], vec![0, 2, 1], vec![1, 2, 0]], vec![2, 2, 2]; "case_1")]
    #[test_case(vec![vec![0, 1, 2, 3], vec![0, 1, 3, 2], vec![2, 3, 1, 0]], vec![2, 3, 2, 2]; "case_2")]
    fn test_correct_anti_plurality(votes: Vec<Vec<usize>>, answer: Vec<usize>) {
        let scorer = AntiPluralityScorer;

        assert_eq!(
            answer,
            scorer.compute_score(&votes.try_into().unwrap()).unwrap()
        );
    }
}
