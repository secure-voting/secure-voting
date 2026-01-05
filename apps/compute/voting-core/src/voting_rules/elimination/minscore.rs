//! Minscore Eliminator implementation.
//!
//! This module defines the [`MinScoreElimination`] struct.

use crate::{prelude::CandidateId, voting_rules::elimination::criterion::EliminationCriterion};

/// Min score eliminator.
///
/// Finds all the candidates with the lowest scores
/// from a vector of scores.
#[derive(Debug, Clone, Copy)]
pub struct MinScoreElimination;

impl EliminationCriterion for MinScoreElimination {
    type Score = Vec<usize>;

    fn eliminate(&self, scores: &Self::Score) -> Vec<CandidateId> {
        let min_score = scores.iter().min().copied().unwrap();
        scores
            .iter()
            .enumerate()
            .filter(|(_, elem)| min_score == **elem)
            .map(|(idx, _)| CandidateId::new(idx))
            .collect()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_single_lowest_score() {
        let scores = vec![0, 1, 2, 1];

        assert_eq!(
            vec![CandidateId::new(0)],
            MinScoreElimination.eliminate(&scores)
        );
    }

    #[test]
    fn test_multiple_lowest_scores() {
        let scores = vec![3, 1, 2, 1];

        assert_eq!(
            vec![CandidateId::new(1), CandidateId::new(3)],
            MinScoreElimination.eliminate(&scores)
        );
    }

    #[test]
    fn test_all_lowest_scores() {
        let scores = vec![1, 1, 1, 1, 1];

        assert_eq!(
            (0..5).map(|x| CandidateId::new(x)).collect::<Vec<_>>(),
            MinScoreElimination.eliminate(&scores)
        );
    }
}
