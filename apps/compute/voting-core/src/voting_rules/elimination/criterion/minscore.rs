//! Minscore Eliminator implementation.
//!
//! This module defines the [`MinScoreElimination`] struct.

use crate::{
    prelude::CandidateId, scorer::Score, voting_rules::elimination::criterion::EliminationCriterion,
};

/// Min score eliminator.
///
/// Finds all the candidates with the lowest scores
/// from a vector of scores.
#[derive(Debug, Clone, Copy)]
pub struct MinScoreElimination;

impl EliminationCriterion for MinScoreElimination {
    type Score = Vec<usize>;

    fn eliminate(&self, scores: &Score<Self::Score>) -> Vec<CandidateId> {
        #[allow(clippy::unwrap_used)]
        let min_score = scores.iter().map(|(score, _)| score).min().unwrap();
        scores
            .iter()
            .filter(|(score, _)| min_score == *score)
            .map(|(_, &id)| id)
            .collect()
    }

    fn new() -> Self {
        Self
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn single_lowest_score() {
        let scores = Score::new(
            vec![0, 1, 2, 1],
            &[
                CandidateId::new(2),
                CandidateId::new(9),
                CandidateId::new(7),
                CandidateId::new(5),
            ],
        );

        assert_eq!(
            vec![CandidateId::new(2)],
            MinScoreElimination.eliminate(&scores)
        );
    }

    #[test]
    fn multiple_lowest_scores() {
        let scores = Score::new(
            vec![3, 1, 2, 1],
            &[
                CandidateId::new(2),
                CandidateId::new(9),
                CandidateId::new(7),
                CandidateId::new(5),
            ],
        );

        assert_eq!(
            vec![CandidateId::new(9), CandidateId::new(5)],
            MinScoreElimination.eliminate(&scores)
        );
    }

    #[test]
    fn all_lowest_scores() {
        let scores = Score::new(
            vec![1, 1, 1, 1, 1],
            &[
                CandidateId::new(2),
                CandidateId::new(9),
                CandidateId::new(7),
                CandidateId::new(5),
                CandidateId::new(15),
            ],
        );

        assert_eq!(
            [2, 9, 7, 5, 15]
                .iter()
                .map(|&x| CandidateId::new(x))
                .collect::<Vec<_>>(),
            MinScoreElimination.eliminate(&scores)
        );
    }
}
