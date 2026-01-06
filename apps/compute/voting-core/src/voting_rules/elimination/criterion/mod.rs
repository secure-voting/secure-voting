//! The elimination criterion module.
//!
//! This module defines the [`EliminationCriterion`] trait.

use crate::profile::CandidateId;

pub mod below_average_score;
pub mod minscore;

/// Criterion on which to eliminate a candidate(-s) from the score.
pub trait EliminationCriterion {
    /// Scoring type to choose the candidate(-s) from.
    type Score;

    /// Find the lowest candidate(-s) from the score.
    fn eliminate(&self, scores: &Self::Score) -> Vec<CandidateId>;
}
