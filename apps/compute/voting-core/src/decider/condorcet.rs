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
