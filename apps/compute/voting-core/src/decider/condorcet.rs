//! Condorcet decider implementation.

use std::convert::Infallible;

use crate::{
    decider::Decider, matrix::CondorcetMatrix, models::candidate_id::CandidateId, scorer::Score,
};

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
        for (row, cand_id) in scores.iter() {
            if row.iter().map(|&elem| usize::from(elem)).sum::<usize>() + 1 == row.len() {
                return Ok(vec![cand_id.clone()]);
            }
        }

        Ok(vec![])
    }

    fn new() -> Self {
        Self
    }
}

#[cfg(test)]
mod tests {
    use crate::matrix::PairwiseMatrix;

    use super::*;

    #[test]
    fn condorcet_winner() {
        let scores = Score::new(
            PairwiseMatrix::try_new(vec![vec![0, 1, 1], vec![0, 0, 0], vec![0, 1, 0]], 1)
                .expect("Pairwise matrix is incorrectly constructed, revise text example")
                .into(),
            &[
                CandidateId::new(42, "A"),
                CandidateId::new(1, "B"),
                CandidateId::new(87, "C"),
            ],
        );

        assert_eq!(
            vec![CandidateId::new(42, "A")],
            CondorcetDecider.decide(&scores).unwrap()
        );
    }

    #[test]
    fn condorcet_cycle() {
        let scores = Score::new(
            PairwiseMatrix::try_new(vec![vec![0, 1, 0], vec![0, 0, 1], vec![1, 0, 0]], 1)
                .expect("Pairwise matrix is incorrectly constructed, revise text example")
                .into(),
            &[
                CandidateId::new(4, "A"),
                CandidateId::new(2, "B"),
                CandidateId::new(67, "C"),
            ],
        );
        let answer: Vec<CandidateId> = vec![];

        assert_eq!(answer, CondorcetDecider.decide(&scores).unwrap());
    }
}
