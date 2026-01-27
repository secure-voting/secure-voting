//! Condorcet matrix type module.
//!
//! This module defines the [`CondorcetMatrix`] type.

use std::fmt::Debug;

use crate::matrix::PairwiseMatrix;

/// Wrapper type over the matrix.
///
/// Enforces the following invariants:
///
/// 1. Matrix has different length rows.
/// 2. Matrix is non-empty
/// 3. Matrix is square.
/// 4. All diagonal elements are 0.
/// 5. All non-diagonal elements are 0/1.
/// 6. For all i != j: matrix\[i\]\[j\] + matrix\[j\]\[i\] = 1.
#[derive(Debug, Clone, PartialEq, Eq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct CondorcetMatrix {
    /// Inner condorcet matrix to be validated.
    matrix: Vec<Vec<bool>>,
}

impl CondorcetMatrix {
    /// Return an iterator over the matrix rows (voters).
    pub fn iter(&self) -> core::slice::Iter<'_, Vec<bool>> {
        self.matrix.iter()
    }
}

impl<'a> IntoIterator for &'a CondorcetMatrix {
    type Item = &'a std::vec::Vec<bool>;
    type IntoIter = std::slice::Iter<'a, std::vec::Vec<bool>>;
    fn into_iter(self) -> Self::IntoIter {
        self.iter()
    }
}

impl From<CondorcetMatrix> for Vec<Vec<bool>> {
    fn from(value: CondorcetMatrix) -> Self {
        value.matrix
    }
}

impl AsRef<[Vec<bool>]> for CondorcetMatrix {
    fn as_ref(&self) -> &[Vec<bool>] {
        &self.matrix
    }
}

impl From<PairwiseMatrix> for CondorcetMatrix {
    #[allow(clippy::needless_range_loop)]
    fn from(matrix: PairwiseMatrix) -> Self {
        let mut result = vec![vec![false; matrix.n()]; matrix.n()];

        for i in 0..result.len() {
            for j in i + 1..result.len() {
                if matrix.beats(i, j) {
                    result[i][j] = true;
                } else if matrix.beats(j, i) {
                    result[j][i] = true;
                }
            }
        }

        CondorcetMatrix { matrix: result }
    }
}
