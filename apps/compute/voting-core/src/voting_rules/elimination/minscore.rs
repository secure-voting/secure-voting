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
