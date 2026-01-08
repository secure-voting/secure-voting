//! The elimination criterion module.
//!
//! This module defines the [`EliminationCriterion`] trait.

use crate::{profile::CandidateId, scorer::Score};

pub mod below_average_score;
pub mod maxscore;
pub mod minscore;

/// Criterion on which to eliminate a candidate(-s) from the score.
pub trait EliminationCriterion {
    /// Scoring type to choose the candidate(-s) from.
    type Score;

    /// Find the lowest candidate(-s) from the score.
    fn eliminate(&self, scores: &Score<Self::Score>) -> Vec<CandidateId>;

    /// Construct a new elimination criterion.
    fn new() -> Self;
}
