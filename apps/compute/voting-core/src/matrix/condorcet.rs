//! Condorcet matrix type module.
//!
//! This module defines the [`CondorcetMatrix`] type and its [`CondorcetMatrixError`] error type.

use std::fmt::Debug;
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
    matrix: Vec<Vec<bool>>,
}

impl CondorcetMatrix {
    /// Construct a CondorcetMatrix without validating its invariants.
    /// Not exposed past this crate.
    ///
    /// SAFETY:
    ///
    /// The caller must ensure the type's invariants, otherwise a panic may occur.
    #[allow(unsafe_code)]
    pub(crate) unsafe fn new_unchecked(matrix: Vec<Vec<usize>>) -> Self {
        CondorcetMatrix {
            matrix: matrix
                .into_iter()
                .map(|row| row.into_iter().map(|elem| elem == 1).collect())
                .collect(),
        }
    }

    /// Return an iterator over the matrix rows (voters).
    pub fn iter(&self) -> core::slice::Iter<'_, Vec<bool>> {
        self.matrix.iter()
    }
}

impl From<CondorcetMatrix> for Vec<Vec<bool>> {
    fn from(value: CondorcetMatrix) -> Self {
        value.matrix
    }
}
