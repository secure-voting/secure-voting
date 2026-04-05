//! Threshold Decider implementation

use std::convert::Infallible;

use crate::{decider::Decider, models::candidate_id::CandidateId, scorer::Score};

/// Majority decider.
///
/// Determines which of the candidates is the best based on their threshold vectors.
#[derive(Default, Debug, Clone, Copy)]
pub struct ThresholdDecider;

impl Decider for ThresholdDecider {
    type Input = Vec<Vec<usize>>;
    type Error = Infallible;

    fn decide(&self, scores: &Score<Self::Input>) -> Result<Vec<CandidateId>, Self::Error> {
        let mut best: Option<&Vec<usize>> = None;
        let mut winners = Vec::new();

        for (score_vec, candidate) in scores.iter() {
            match best {
                None => {
                    best = Some(score_vec);
                    winners.push(*candidate);
                }
                Some(best_vec) => match score_vec.cmp(best_vec) {
                    std::cmp::Ordering::Less => {
                        best = Some(score_vec);
                        winners.clear();
                        winners.push(*candidate);
                    }
                    std::cmp::Ordering::Equal => {
                        winners.push(*candidate);
                    }
                    std::cmp::Ordering::Greater => {}
                },
            }
        }

        Ok(winners)
    }

    fn new() -> Self {
        Self {}
    }
}
