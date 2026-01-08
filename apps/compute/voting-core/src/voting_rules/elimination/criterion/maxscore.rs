//! Maxscore Eliminator implementation.
//!
//! This module defines the [`MaxScoreElimination`] struct.

use crate::{
    prelude::CandidateId, scorer::Score, voting_rules::elimination::criterion::EliminationCriterion,
};

/// Max score eliminator.
///
/// Finds all the candidates with the highest scores
/// from a vector of scores.
#[derive(Debug, Clone, Copy)]
pub struct MaxScoreElimination;

impl EliminationCriterion for MaxScoreElimination {
    type Score = Vec<usize>;

    fn eliminate(&self, scores: &Score<Self::Score>) -> Vec<CandidateId> {
        #[allow(clippy::unwrap_used)]
        let max_score = scores.iter().map(|(s, _)| s).max().unwrap();
        scores
            .iter()
            .filter(|(score, _)| max_score == *score)
            .map(|(_, &id)| id)
            .collect()
    }

    fn new() -> Self {
        Self
    }
}
