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
            .map(|(_, id)| id)
            .cloned()
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
        let candidates = vec![
            CandidateId::new(2, "A"),
            CandidateId::new(9, "B"),
            CandidateId::new(7, "C"),
            CandidateId::new(5, "D"),
        ];

        let scores = Score::new(vec![0usize, 1, 2, 1], &candidates);

        assert_eq!(
            vec![CandidateId::new(2, "A")],
            MinScoreElimination.eliminate(&scores)
        );
    }

    #[test]
    fn multiple_lowest_scores() {
        let candidates = vec![
            CandidateId::new(2, "A"),
            CandidateId::new(9, "B"),
            CandidateId::new(7, "C"),
            CandidateId::new(5, "D"),
        ];

        let scores = Score::new(vec![3usize, 1, 2, 1], &candidates);

        assert_eq!(
            vec![CandidateId::new(9, "B"), CandidateId::new(5, "D"),],
            MinScoreElimination.eliminate(&scores)
        );
    }

    #[test]
    fn all_lowest_scores() {
        let candidates = vec![
            CandidateId::new(2, "A"),
            CandidateId::new(9, "B"),
            CandidateId::new(7, "C"),
            CandidateId::new(5, "D"),
            CandidateId::new(15, "E"),
        ];

        let scores = Score::new(vec![1usize, 1, 1, 1, 1], &candidates);

        assert_eq!(candidates, MinScoreElimination.eliminate(&scores));
    }
}
