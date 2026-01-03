//! Pairwise voting result maitrx type module.
//!
//! This module defines the [`PairwiseMatrix`] type and its [`PairwiseMatrixError`] error type.

use rayon::prelude::*;

use crate::{matrix::condorcet::CondorcetMatrix, profile::Profile};

/// Pairwise voting result matrix.
///
/// Ensures the following invariants are upheld:
///
/// 1. It is a matrix = rows of the same length
/// 2. It is a non-empty matrix
/// 3. It is a square-matrix
/// 4. The diagonal is zero-ed out
/// 5. The following identity is held: matrix\[i\]\[j\] + matrix\[j\]\[i\] = n, if i != j and n is the number of voters.
pub struct PairwiseMatrix {
    /// Underlying voting result matrix type
    matrix: Vec<Vec<usize>>,
}

impl PairwiseMatrix {
    /// Return the square matrix's row/col count.
    pub fn n(&self) -> usize {
        self.matrix.len()
    }

    /// Check whether the i-th candidate strongly beats the j-th candidate
    /// in a head-to-head vote-off.
    pub fn beats(&self, i: usize, j: usize) -> bool {
        self.matrix[i][j] > self.matrix[j][i]
    }
}

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

impl From<&Profile> for PairwiseMatrix {
    fn from(profile: &Profile) -> Self {
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

        PairwiseMatrix {
            matrix: vote_counts,
        }
    }
}

impl From<PairwiseMatrix> for CondorcetMatrix {
    #[allow(clippy::needless_range_loop)]
    fn from(matrix: PairwiseMatrix) -> Self {
        let mut result = vec![vec![0; matrix.n()]; matrix.n()];

        for i in 0..result.len() {
            for j in i + 1..result.len() {
                if matrix.beats(i, j) {
                    result[i][j] = 1;
                } else if matrix.beats(j, i) {
                    result[j][i] = 1;
                }
            }
        }

        // Unsafe code here is justified, because all the invariants are held by the library.
        #[allow(unsafe_code)]
        unsafe {
            CondorcetMatrix::new_unchecked(result)
        }
    }
}
