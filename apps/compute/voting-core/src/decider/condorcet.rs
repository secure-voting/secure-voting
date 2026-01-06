//! Condorcet decider implementation.

use std::convert::Infallible;

use crate::{decider::Decider, matrix::CondorcetMatrix, profile::CandidateId};

/// Condorcet decider.
///
/// Chooses a Condorcet winner from the table, if there is one.
/// Returns an empty winner set otherwise.
pub struct CondorcetDecider;

impl Decider for CondorcetDecider {
    type Input = CondorcetMatrix;

    type Error = Infallible;

    fn decide(&self, scores: &Self::Input) -> Result<Vec<CandidateId>, Self::Error> {
        for (idx, row) in scores.iter().enumerate() {
            if row.iter().map(|&elem| elem as usize).sum::<usize>() + 1 == row.len() {
                return Ok(vec![CandidateId::new(idx)]);
            }
        }

        Ok(vec![])
    }
}

// Unsafe code because CondorcetMatrix is constructed from a Vec<Vec<usize>>
// in an "unsafe" matter (invariants are not upheld).
#[allow(unsafe_code)]
#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_condorcet_winner() {
        let scores = unsafe {
            CondorcetMatrix::new_unchecked(vec![vec![0, 1, 1], vec![0, 0, 0], vec![0, 1, 0]])
        };

        assert_eq!(
            vec![CandidateId::new(0)],
            CondorcetDecider.decide(&scores).unwrap()
        );
    }

    #[test]
    fn test_condorcet_cycle() {
        let scores = unsafe {
            CondorcetMatrix::new_unchecked(vec![vec![0, 1, 0], vec![0, 0, 1], vec![1, 0, 0]])
        };
        let answer: Vec<CandidateId> = vec![];

        assert_eq!(answer, CondorcetDecider.decide(&scores).unwrap());
    }
}
