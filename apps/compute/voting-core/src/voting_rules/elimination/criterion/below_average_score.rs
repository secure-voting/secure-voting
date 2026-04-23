//! `BelowAverageElimination` module.

use crate::{
    prelude::CandidateId, scorer::Score, voting_rules::elimination::criterion::EliminationCriterion,
};

/// Chooses candidates with scores below the average across the scoring results.
#[derive(Debug, Clone, Copy)]
pub struct BelowAverageElimination;

impl EliminationCriterion for BelowAverageElimination {
    type Score = Vec<usize>;

    fn eliminate(&self, scores: &Score<Self::Score>) -> Vec<CandidateId> {
        let score_len = scores.score().len();
        let score_sum = scores.iter().map(|(score, _)| score).sum();

        scores
            .iter()
            .filter(|(score, _)| *score * score_len < score_sum)
            .map(|(_, cand_id)| cand_id)
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
    use test_case::test_case;

    fn ids(value: &[CandidateId]) -> Vec<usize> {
        value.iter().map(|x| x.clone().get_id()).collect()
    }

    #[test_case(vec![0, 4, 1], &[0, 2]; "clear winner gap")]
    #[test_case(vec![0, 2, 1], &[0]; "no clear winner")]
    #[test_case(vec![1, 1, 1], &[]; "even distribution")]
    fn test(scores: Vec<usize>, answer: &[usize]) {
        assert_eq!(
            answer,
            ids(&BelowAverageElimination.eliminate(&Score::new(
                scores,
                &[
                    CandidateId::new(0, "a"),
                    CandidateId::new(1, "b"),
                    CandidateId::new(2, "c")
                ]
            )))
        );
    }
}
