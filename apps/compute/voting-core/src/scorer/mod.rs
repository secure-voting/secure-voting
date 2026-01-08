//! Scoring rule applied to a voting profile prior to winner selection.
//!
//! This module defines a [`Scorer`] trait and provides pre-packaged implementations.

use std::fmt::Debug;

use crate::profile::{CandidateId, Profile};

pub mod anti_plurality;
pub mod approval;
pub mod borda;
pub mod condorcet;
pub mod copeland;
pub mod minmax;
pub mod plurality;
pub mod simpson;

/// The score type to be used by Scorers.
pub struct Score<T> {
    scores: T,
    candidates: Vec<CandidateId>,
}

impl<T> Score<T> {
    pub fn new(scores: T, candidates: &[CandidateId]) -> Self {
        Self {
            scores,
            candidates: candidates.to_vec(),
        }
    }

    pub fn score(&self) -> &T {
        &self.scores
    }

    pub fn candidates(&self) -> &[CandidateId] {
        &self.candidates
    }
}

/// Computes the scores for the profile of voters.
pub trait Scorer {
    /// Output type produced by this scorer.
    ///
    /// Usually a matrix or a vector.
    type Output;

    /// Error returned when scoring can't be done.
    ///
    /// Use [`std::convert::Infallible`] if this step cannot fail.
    type Error: Debug;

    /// Scores the voting profile.
    fn compute_score(&self, profile: &Profile) -> Result<Score<Self::Output>, Self::Error>;
}
