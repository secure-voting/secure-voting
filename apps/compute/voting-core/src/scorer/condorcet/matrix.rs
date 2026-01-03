//! Condorcet Matrix type implementation.
//!
//! This module defines the [`CondorcetMatrix`] type as well as its error type [`CondorcetMatrixError`].

use thiserror::Error;

/// Wrapper type over the matrix.
///
/// Enforces the following invariants:
///
/// 1. Matrix has different length rows.
/// 2. Matrix is non-empty
/// 3. Matrix is square.
/// 4. All diagonal elements are 0.
/// 5. All non-diagonal elements are 0/1.
/// 6. For all i != j: matrix[i][j] + matrix[j][i] = 1.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct CondorcetMatrix {
    /// Inner condorcet matrix to be validated.
    matrix: Vec<Vec<usize>>,
}

/// An error type for the Condorcet matrix creation.
#[derive(Error, Debug, PartialEq, Eq)]
pub enum CondorcetMatrixError {
    /// Not actually a matrix, different row lengths.
    #[error("Not a matrix")]
    NonMatrix,
    /// Empty matrix
    #[error("Matrix is empty")]
    EmptyMatrix,
    /// Matrix is not square.
    #[error("Matrix is not a square-matrix")]
    NonSquareMatrix,
    /// Some diagonal elements are not zero
    #[error("Non-zero diagonal element found")]
    NonZeroDiagonal,
    /// Some element is not 0/1
    #[error("There is an element not equal to 0 or 1")]
    NonBinaryElement,
    /// Matrix "anti-symmetric" identity doesn't hold
    #[error("matrix[i][j] + matrix[j][i] = 1 doesn't hold")]
    NonAntiSymmetric,
}

impl CondorcetMatrix {
    /// Construct a validated Condorcet Matrix.
    ///
    /// Validates all the type invariants. See [`CondorcetMatrix`] docs.
    #[allow(clippy::needless_range_loop)]
    pub fn try_new(matrix: Vec<Vec<usize>>) -> Result<CondorcetMatrix, CondorcetMatrixError> {
        if matrix.is_empty() {
            return Err(CondorcetMatrixError::EmptyMatrix);
        }

        let row_0_len = matrix[0].len();

        if matrix.iter().skip(1).any(|row| row.len() != row_0_len) {
            return Err(CondorcetMatrixError::NonMatrix);
        }

        let cols = row_0_len;
        let rows = matrix.len();

        if cols != rows {
            return Err(CondorcetMatrixError::NonSquareMatrix);
        }

        if matrix
            .iter()
            .enumerate()
            .map(|(idx, row)| row[idx])
            .any(|elem| elem != 0)
        {
            return Err(CondorcetMatrixError::NonZeroDiagonal);
        }

        if matrix
            .iter()
            .flat_map(|row| row.iter())
            .any(|&elem| elem > 1)
        {
            return Err(CondorcetMatrixError::NonBinaryElement);
        }

        for i in 0..rows {
            for j in i + 1..rows {
                if matrix[i][j] + matrix[j][i] != 1 {
                    return Err(CondorcetMatrixError::NonAntiSymmetric);
                }
            }
        }

        Ok(CondorcetMatrix { matrix })
    }

    /// Construct a CondorcetMatrix without validating its invariants.
    /// Not exposed past this crate.
    ///
    /// SAFETY:
    ///
    /// The caller must ensure the type's invariants, otherwise a panic may occur.
    #[allow(unsafe_code)]
    pub(crate) unsafe fn new_unchecked(matrix: Vec<Vec<usize>>) -> Self {
        CondorcetMatrix { matrix }
    }

    /// Return an iterator over the matrix rows (voters).
    pub fn iter(&self) -> core::slice::Iter<'_, Vec<usize>> {
        self.matrix.iter()
    }
}

impl From<CondorcetMatrix> for Vec<Vec<usize>> {
    fn from(value: CondorcetMatrix) -> Self {
        value.matrix
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use test_case::test_case;

    #[test]
    fn accepts_single_candidate_matrix() {
        let matrix = vec![vec![0]];
        let cm = CondorcetMatrix::try_new(matrix.clone()).unwrap();
        assert_eq!(Vec::<Vec<usize>>::from(cm), matrix);
    }

    #[test]
    fn accepts_simple_valid_matrix() {
        let matrix = vec![vec![0, 1], vec![0, 0]];

        assert!(CondorcetMatrix::try_new(matrix).is_ok());
    }

    #[test]
    fn accepts_larger_valid_matrix() {
        let matrix = vec![vec![0, 1, 0], vec![0, 0, 1], vec![1, 0, 0]];

        assert!(CondorcetMatrix::try_new(matrix).is_ok());
    }

    use CondorcetMatrixError::*;

    #[test_case(vec![], EmptyMatrix; "empty matrix")]
    #[test_case(vec![vec![0, 1], vec![0]], NonMatrix; "non matrix")]
    #[test_case(vec![vec![0, 1, 0], vec![1, 0, 0]], NonSquareMatrix; "non sqaure matrix")]
    #[test_case(vec![vec![0, 1], vec![1, 1]], NonZeroDiagonal; "non zero diagonal")]
    #[test_case(vec![vec![0, 2], vec![0, 0]], NonBinaryElement; "non binary element")]
    #[test_case(vec![vec![0, 1], vec![1, 0]], NonAntiSymmetric; "non-antisymmetric pair")]
    fn test_incorrect_invariant_violated(input: Vec<Vec<usize>>, error: CondorcetMatrixError) {
        assert_eq!(CondorcetMatrix::try_new(input).unwrap_err(), error);
    }
}
