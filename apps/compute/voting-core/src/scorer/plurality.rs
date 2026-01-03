//! Plurality scorer implementation.
//!
//! Votes are counted only for the first candidate.

use std::convert::Infallible;

use rayon::prelude::*;

use crate::{profile::Profile, scorer::Scorer};

/// Plurality scorer.
///
/// Gives one point to the top candidate.
pub struct PluralityScorer;

impl Scorer for PluralityScorer {
    type Error = Infallible;
    type Output = Vec<usize>;

    fn compute_score(&self, profile: &Profile) -> Result<Self::Output, Self::Error> {
        let n_voters = profile.n_voters();
        let n_candidates = profile.n_candidates();

        Ok((0..n_voters)
            .into_par_iter()
            .map(|i| {
                let mut tmp = vec![0; n_candidates];
                tmp[profile[i][0].into_inner()] = 1;

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
}
