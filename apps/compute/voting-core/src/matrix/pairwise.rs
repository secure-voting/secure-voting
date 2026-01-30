//! Pairwise voting result maitrx type module.
//!
//! This module defines the [`PairwiseMatrix`] type and its [`PairwiseMatrixError`] error type.

use rayon::prelude::*;
use thiserror::Error;

use crate::models::profile::Profile;

/// Pairwise voting result matrix.
///
/// Ensures the following invariants are upheld:
///
/// 1. It is a matrix = rows of the same length
/// 2. It is a non-empty matrix
/// 3. It is a square-matrix
/// 4. The diagonal is zero-ed out
/// 5. The following identity is held: matrix\[i\]\[j\] + matrix\[j\]\[i\] = n, if i != j and n is the number of voters.
#[derive(Debug, Clone, PartialEq, Eq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct PairwiseMatrix {
    /// Underlying voting result matrix type
    matrix: Vec<Vec<usize>>,
    /// The count of voters in this result
    n_voters: usize,
}

/// Pairwise matrix error type.
///
/// Returned when some invariants are not upheld when creating the matrix.
#[derive(Error, Debug, PartialEq)]
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
    ///
    /// # Errors
    ///
    /// A `PairwiseMatrixError` is returned if one of the type's invariants is not upheld:
    ///
    /// 1. Passed matrix is not a matrix at all.
    /// 2. Matrix is empty.
    /// 3. Matrix is not square.
    /// 4. The diagonal has non-zero elements.
    /// 5. matrix\[i\][\j\] + matrix\[j\]\[i\] != n for some i != j and n = ``n_voters``
    pub fn try_new(matrix: Vec<Vec<usize>>, n_voters: usize) -> Result<Self, PairwiseMatrixError> {
        if matrix.is_empty() {
            return Err(PairwiseMatrixError::EmptyMatrix);
        }

        let row_0_len = matrix[0].len();

        if matrix
            .iter()
            .skip(1)
            .map(std::vec::Vec::len)
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
    #[must_use]
    pub fn n(&self) -> usize {
        self.matrix.len()
    }

    /// Return the voter count.
    #[must_use]
    pub fn n_voters(&self) -> usize {
        self.n_voters
    }

    /// Check whether the i-th candidate strongly beats the j-th candidate
    /// in a head-to-head vote-off.
    #[must_use]
    pub fn beats(&self, i: usize, j: usize) -> bool {
        self.matrix[i][j] > self.matrix[j][i]
    }

    /// Subtract 2 usizes into an isize without possible overflows.
    #[inline]
    fn safe_sub_usize_to_isize(a: usize, b: usize) -> isize {
        if a >= b {
            (a - b).cast_signed()
        } else {
            -(b - a).cast_signed()
        }
    }

    /// Return the margin of win/loss between candidates i and j.
    #[must_use]
    pub fn margin(&self, i: usize, j: usize) -> isize {
        Self::safe_sub_usize_to_isize(self.matrix[i][j], self.matrix[j][i])
    }

    /// Return an iterator over the rows of the matrix.
    pub fn iter(&self) -> core::slice::Iter<'_, Vec<usize>> {
        self.matrix.iter()
    }
}

impl<'a> IntoIterator for &'a PairwiseMatrix {
    type Item = &'a std::vec::Vec<usize>;
    type IntoIter = std::slice::Iter<'a, std::vec::Vec<usize>>;
    fn into_iter(self) -> Self::IntoIter {
        self.iter()
    }
}

/// Helper function to sum 2 matrices of the same size.
///
/// # Panics
///
/// Matrices should be non-empty and same sized.
fn sum_matrix(mut matrix_a: Vec<Vec<usize>>, matrix_b: &[Vec<usize>]) -> Vec<Vec<usize>> {
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
                |a, b| sum_matrix(a, &b),
            );

        PairwiseMatrix {
            matrix: vote_counts,
            n_voters: profile.n_voters(),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use test_case::test_case;

    #[test]
    fn accepts_simple_valid_matrix() {
        let matrix = vec![vec![0, 1], vec![0, 0]];
        assert!(PairwiseMatrix::try_new(matrix, 1).is_ok());
    }

    #[test]
    fn accepts_larger_valid_matrix() {
        let matrix = vec![vec![0, 1, 0], vec![0, 0, 1], vec![1, 0, 0]];

        assert!(PairwiseMatrix::try_new(matrix, 1).is_ok());
    }

    use PairwiseMatrixError::*;

    #[test_case(vec![], 0, &EmptyMatrix; "empty matrix")]
    #[test_case(vec![vec![0, 1], vec![0]], 0, &NonMatrix; "non matrix")]
    #[test_case(vec![vec![0, 1, 0], vec![1, 0, 0]], 0, &NonSquareMatrix; "non sqaure matrix")]
    #[test_case(vec![vec![0, 1], vec![1, 1]], 0, &NonZeroDiagonal; "non zero diagonal")]
    #[test_case(vec![vec![0, 1], vec![1, 0]], 1, &NonAntiSymmetric; "non-antisymmetric pair")]
    fn test_incorrect_invariant_violated(
        input: Vec<Vec<usize>>,
        n_voters: usize,
        error: &PairwiseMatrixError,
    ) {
        assert_eq!(
            PairwiseMatrix::try_new(input, n_voters).expect_err("Test should have failed."),
            *error
        );
    }
}
