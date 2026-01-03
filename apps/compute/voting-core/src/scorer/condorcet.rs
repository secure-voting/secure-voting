//! Condorcet scorer implementation.
//!
//! Calculates all head-to-head candidates' scores.

use std::convert::Infallible;

use rayon::prelude::*;

use crate::{matrix::CondorcetMatrix, profile::Profile, scorer::Scorer};

/// Condorcet Scorer type.
///
/// Gives points only of the candidate wins in a head-to-head.
pub struct CondorcetScorer;

/// Helper function to sum 2 matrices of the same size.
///
/// Assumes that matrices actually have the same size.
/// Panics if they are different sizes.
fn sum_matrix(matrix_a: Vec<Vec<usize>>, matrix_b: Vec<Vec<usize>>) -> Vec<Vec<usize>> {
    assert_eq!(matrix_a.len(), matrix_b.len());
    assert_eq!(matrix_a[0].len(), matrix_b[0].len());

    let mut result = vec![vec![0; matrix_a.len()]; matrix_a.len()];

    for i in 0..matrix_a.len() {
        for j in 0..matrix_b[0].len() {
            result[i][j] = matrix_a[i][j] + matrix_b[i][j];
        }
    }

    result
}

/// Helper function to convert from raw matrix vote counts to the matrix of 0/1.
///
/// Assumes the matrix is a square matrix, panics otherwise.
fn convert_to_condorcet(matrix: Vec<Vec<usize>>) -> Vec<Vec<usize>> {
    assert_eq!(matrix.len(), matrix[0].len());
    let mut result = vec![vec![0; matrix.len()]; matrix.len()];

    for i in 0..result.len() {
        for j in i + 1..result.len() {
            if matrix[i][j] > matrix[j][i] {
                result[i][j] = 1;
            } else if matrix[i][j] < matrix[j][i] {
                result[j][i] = 1;
            }
        }
    }

    result
}

impl Scorer for CondorcetScorer {
    /// Matrix of head-to-head preferences.
    ///
    /// Each row describes a candidate.
    /// If there is a zero on the i-th position, then current candidate loses to i-th in a head-to-head.
    /// If there is a one on the i-th position, then current candidate wins to i-th in a head-to-head.
    type Output = CondorcetMatrix;

    type Error = Infallible;

    fn compute_score(&self, profile: &Profile) -> Result<Self::Output, Self::Error> {
        let n_candidates = profile.n_candidates();
        let n_voters = profile.n_voters();

        let vote_counts = (0..n_voters)
            .into_par_iter()
            .map(|voter_id| {
                let mut matrix = vec![vec![0; n_candidates]; n_candidates];

                for i in 0..n_candidates {
                    for j in i + 1..n_candidates {
                        matrix[profile[voter_id][i].into_inner()]
                            [profile[voter_id][j].into_inner()] = 1;
                    }
                }

                matrix
            })
            .reduce(|| vec![vec![0; n_candidates]; n_candidates], sum_matrix);

        #[allow(unsafe_code)]
        // The construction process ensures the invariants of the matrix are upheld
        unsafe {
            Ok(CondorcetMatrix::new_unchecked(convert_to_condorcet(
                vote_counts,
            )))
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_correct_condorcet_matrix_single_vote() {
        let votes = vec![vec![1, 2, 0, 3]];
        let answer = vec![
            vec![false, false, false, true],
            vec![true, false, true, true],
            vec![true, false, false, true],
            vec![false, false, false, false],
        ];
        let profile = votes.try_into().unwrap();

        assert_eq!(
            answer,
            Into::<Vec<Vec<bool>>>::into(CondorcetScorer.compute_score(&profile).unwrap())
        );
    }

    #[test]
    fn test_correct_condorcet_matrix_multiple_votes() {
        let votes = vec![vec![1, 2, 0, 3], vec![3, 0, 2, 1], vec![0, 2, 1, 3]];
        let answer = vec![
            vec![false, true, true, true],
            vec![false, false, false, true],
            vec![false, true, false, true],
            vec![false, false, false, false],
        ];
        let profile = votes.try_into().unwrap();

        assert_eq!(
            answer,
            Into::<Vec<Vec<bool>>>::into(CondorcetScorer.compute_score(&profile).unwrap())
        );
    }
}
