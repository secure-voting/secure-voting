//! Condorcet decider implementation.

use std::convert::Infallible;

use crate::{decider::Decider, matrix::CondorcetMatrix, profile::CandidateId, scorer::Score};

/// Condorcet decider.
///
/// Chooses a Condorcet winner from the table, if there is one.
/// Returns an empty winner set otherwise.
#[derive(Debug, Clone, Copy)]
pub struct CondorcetDecider;

impl Decider for CondorcetDecider {
    type Input = CondorcetMatrix;

    type Error = Infallible;

    fn decide(&self, scores: &Score<Self::Input>) -> Result<Vec<CandidateId>, Self::Error> {
        for (row, &cand_id) in scores.iter() {
            if row.iter().map(|&elem| elem as usize).sum::<usize>() + 1 == row.len() {
                return Ok(vec![cand_id]);
            }
        }

        Ok(vec![])
    }

    fn new() -> Self {
        Self
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
        let scores = Score::new(
            unsafe {
                CondorcetMatrix::new_unchecked(vec![vec![0, 1, 1], vec![0, 0, 0], vec![0, 1, 0]])
            },
            &[
                CandidateId::new(42),
                CandidateId::new(1),
                CandidateId::new(87),
            ],
        );

        assert_eq!(
            vec![CandidateId::new(42)],
            CondorcetDecider.decide(&scores).unwrap()
        );
    }

    #[test]
    fn test_condorcet_cycle() {
        let scores = Score::new(
            unsafe {
                CondorcetMatrix::new_unchecked(vec![vec![0, 1, 0], vec![0, 0, 1], vec![1, 0, 0]])
            },
            &[
                CandidateId::new(4),
                CandidateId::new(2),
                CandidateId::new(67),
            ],
        );
        let answer: Vec<CandidateId> = vec![];

        assert_eq!(answer, CondorcetDecider.decide(&scores).unwrap());
    }
}
