//! Q-Paretian Family of algorithms' module.

pub mod strong_majority;
pub mod strong_plurality;
pub mod strongest_majority;

use std::ops::Index;

use thiserror::Error;

use crate::models::{profile::Profile, ranking::RankingBallot};

/// `PositionMatrix` creation error type.
#[derive(Debug, Error)]
pub enum PositionMatrixError {
    /// Matrix has no rows.
    #[error("Matrix is empty")]
    Empty,
    /// Matrix has rows of different lengths.
    #[error("Rows of different lengths")]
    NonMatrix,
}

/// Position matrix, pos\[i\]\[j\] = k means
/// i-th voter placed j-th alternative on the k-th place (counting from 0).
///
/// Can only be constructed fallibly via `TryFrom<Vec<Vec<usize>>>`.
pub struct PositionMatrix {
    /// Matrix of positions.
    matrix: Vec<Vec<usize>>,
}

impl PositionMatrix {
    /// Get the number of alternatives.
    #[must_use]
    pub fn m(&self) -> usize {
        self.matrix[0].len()
    }
}

impl Index<usize> for PositionMatrix {
    type Output = Vec<usize>;

    fn index(&self, index: usize) -> &Self::Output {
        &self.matrix[index]
    }
}

impl TryFrom<Vec<Vec<usize>>> for PositionMatrix {
    type Error = PositionMatrixError;

    fn try_from(value: Vec<Vec<usize>>) -> Result<Self, Self::Error> {
        if value.is_empty() {
            return Err(PositionMatrixError::Empty);
        }

        if value
            .iter()
            .map(std::vec::Vec::len)
            .any(|x| x != value[0].len())
        {
            return Err(PositionMatrixError::NonMatrix);
        }

        Ok(Self { matrix: value })
    }
}

/// Build a position matrix, pos\[voter\]\[alternative\] = position (0 = best).
///
/// # Panics
///
/// When the profile has an already invalidated candidate (removed) in the ballots, `Profile::index_of` will panic.
#[must_use]
#[allow(
    clippy::expect_used,
    reason = "Profile can't be created invalid, so expect wouldn't fail"
)]
pub fn build_pos(profile: &Profile<RankingBallot>) -> PositionMatrix {
    let n = profile.n_voters();
    let m = profile.n_candidates();

    let mut pos = vec![vec![m; m]; n];

    for i in 0..n {
        for (p, &a) in profile[i].iter().enumerate() {
            pos[i][profile
                .index_of(&a)
                .expect("Ballot contains invalidated voter")] = p;
        }
    }

    PositionMatrix::try_from(pos)
        .expect("Position matrix should reliably be created from a valid Profile")
}

/// `T_i^q` alternatives that are among the top (q+1) for voter i.
#[must_use]
pub fn t_1_q(i: usize, q: usize, pos: &PositionMatrix) -> Vec<usize> {
    let m = pos.m();
    (0..m).filter(|&a| pos[i][a] <= q).collect()
}

/// Intersection of `T_i^q` over the members i of the coalition I.
#[must_use]
pub fn t_i_q_intersection(coalition: &[usize], q: usize, pos: &PositionMatrix) -> Vec<usize> {
    let m = pos.m();
    let mut counts = vec![0; m];
    for &ind in coalition {
        for a in 0..m {
            if pos[ind][a] <= q {
                counts[a] += 1;
            }
        }
    }

    counts
        .into_iter()
        .enumerate()
        .filter_map(|(a, c)| if c == coalition.len() { Some(a) } else { None })
        .collect()
}
