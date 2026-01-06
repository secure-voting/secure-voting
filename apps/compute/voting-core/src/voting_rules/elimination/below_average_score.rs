//! BelowAverageElimination module.

use crate::{prelude::CandidateId, voting_rules::elimination::criterion::EliminationCriterion};

/// Chooses candidates with scores below the average across the scoring results.
pub struct BelowAverageElimination;

impl EliminationCriterion for BelowAverageElimination {
    type Score = Vec<usize>;

    fn eliminate(&self, scores: &Self::Score) -> Vec<CandidateId> {
        let avg = scores.iter().sum::<usize>() / scores.len();

        scores
            .iter()
            .enumerate()
            .filter(|(_, score)| **score < avg)
            .map(|(idx, _)| CandidateId::new(idx))
            .collect()
    }
}
