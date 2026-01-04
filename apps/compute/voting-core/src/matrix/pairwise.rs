//! Pairwise voting result maitrx type module.
//!
//! This module defines the [`PairwiseMatrix`] type and its [`PairwiseMatrixError`] error type.

use rayon::prelude::*;
use thiserror::Error;

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
    /// The count of voters in this result
    n_voters: usize,
}

/// Pairwise matrix error type.
///
/// Returned when some invariants are not upheld when creating the matrix.
#[derive(Error, Debug)]
pub enum PairwiseMatrixError {
    /// If the matrix has 0 rows.
    #[error("Matrix is empty")]
    EmptyMatrix,
    /// Matrix has rows of different lengths.
    #[error("Some rows have different lengths")]
    NonMatrix,
    /// Matrix is not square.
    #[error("Matrix is not square")]
    NonSquareMatrix,
    /// Some diagonal elements are not zero.
    #[error("Diagonal element is not zero")]
    NonZeroDiagonal,
    /// matrix\[i\]\[j\] + matrix\[j\]\[i\] = n doesn't hold.
    #[error("Matrix is not anti-symmetric")]
    NonAntiSymmetric,
}

impl PairwiseMatrix {
    /// Create a validatied pairwise matrix.
    ///
    /// Ensures the invariants of the type are upheld. Otherwise [`PairwiseMatrixError`] is returned.
    pub fn try_new(matrix: Vec<Vec<usize>>, n_voters: usize) -> Result<Self, PairwiseMatrixError> {
        if matrix.is_empty() {
            return Err(PairwiseMatrixError::EmptyMatrix);
        }

        let row_0_len = matrix[0].len();

        if matrix
            .iter()
            .skip(1)
            .map(|row| row.len())
            .any(|row_len| row_len != row_0_len)
        {
            return Err(PairwiseMatrixError::NonMatrix);
        }

        let rows = matrix.len();
        let cols = row_0_len;

        if rows != cols {
            return Err(PairwiseMatrixError::NonSquareMatrix);
        }

        if matrix
            .iter()
            .enumerate()
            .map(|(idx, row)| row[idx])
            .any(|elem| elem != 0)
        {
            return Err(PairwiseMatrixError::NonZeroDiagonal);
        }

        #[allow(clippy::needless_range_loop)]
        for i in 0..rows {
            for j in i + 1..rows {
                if matrix[i][j] + matrix[j][i] != n_voters {
                    return Err(PairwiseMatrixError::NonAntiSymmetric);
                }
            }
        }

        Ok(Self { matrix, n_voters })
    }

    /// Return the square matrix's row/col count.
    pub fn n(&self) -> usize {
        self.matrix.len()
    }

    /// Return the voter count.
    pub fn n_voters(&self) -> usize {
        self.n_voters
    }

    /// Check whether the i-th candidate strongly beats the j-th candidate
    /// in a head-to-head vote-off.
    pub fn beats(&self, i: usize, j: usize) -> bool {
        self.matrix[i][j] > self.matrix[j][i]
    }
}

/// Helper function to sum 2 matrices of the same size.
///
/// SAFETY:
///
/// Matrices should be non-empty and same sized.
/// Otherwise an Out-of-bounds panic can occur.
#[allow(unsafe_code)]
unsafe fn sum_matrix(mut matrix_a: Vec<Vec<usize>>, matrix_b: Vec<Vec<usize>>) -> Vec<Vec<usize>> {
    assert_eq!(matrix_a.len(), matrix_b.len());
    assert_eq!(matrix_a[0].len(), matrix_b[0].len());

    for i in 0..matrix_a.len() {
        for j in 0..matrix_b[0].len() {
            matrix_a[i][j] += matrix_b[i][j];
        }
    }

    matrix_a
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
            .reduce(
                || vec![vec![0; n_candidates]; n_candidates],
                #[allow(unsafe_code)]
                |a, b| unsafe { sum_matrix(a, b) },
            );

        PairwiseMatrix {
            matrix: vote_counts,
            n_voters: profile.n_voters(),
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
