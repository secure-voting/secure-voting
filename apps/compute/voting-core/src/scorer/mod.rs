//! Scoring rule applied to a voting profile prior to winner selection.
//!
//! This module defines a [`Scorer`] trait and provides pre-packaged implementations.

use std::fmt::Debug;

use crate::models::{candidate_id::CandidateId, profile::Profile};

pub mod anti_plurality;
pub mod approval;
pub mod borda;
pub mod condorcet;
pub mod copeland;
pub mod minmax;
pub mod plurality;
pub mod simpson;
pub mod threshold;

pub mod zip;

/// The score type to be used by Scorers.
#[derive(Debug, Clone)]
pub struct Score<T> {
    /// Scores of the candidates.
    scores: Vec<T>,
    /// A list of candidates in this scoring.
    candidates: Vec<CandidateId>,
}

impl<T> Score<T> {
    /// Return a new Score instance.
    pub fn new(scores: Vec<T>, candidates: &[CandidateId]) -> Self {
        Self {
            scores,
            candidates: candidates.to_vec(),
        }
    }

    /// Get a non-owning view of the scores.
    pub fn score(&self) -> &[T] {
        &self.scores
    }

    /// Consume self and get scores.
    pub fn consume_score(self) -> Vec<T> {
        self.scores
    }

    /// Get a non-owning view of the candidates.
    pub fn candidates(&self) -> &[CandidateId] {
        &self.candidates
    }

    /// Get an iterator over pairs of (score, candidate) in the scores.
    pub fn iter<'a>(&'a self) -> impl Iterator<Item = (&'a T, &'a CandidateId)> {
        let scores = self.score();
        debug_assert_eq!(scores.len(), self.candidates.len());

        scores.iter().zip(self.candidates.iter())
    }
}

impl<T: Clone + PartialOrd + Ord> Score<T> {
    /// Get a range of all the scores
    pub fn value_range(self) -> Vec<T> {
        let mut v: Vec<T> = self.consume_score();
        v.sort_unstable();
        v.dedup();
        v.reverse();

        v
    }
}

/// Computes the scores for the profile of voters.
pub trait Scorer<Ballot> {
    /// Output type produced by this scorer.
    ///
    /// Usually a matrix or a vector.
    type Output;

    /// Error returned when scoring can't be done.
    ///
    /// Use [`std::convert::Infallible`] if this step cannot fail.
    type Error: Debug;

    /// Scores the voting profile.
    ///
    /// # Errors
    ///
    /// An error is returned if the scoring step can fail.
    /// Usually happens due to invariants of the scoring step
    /// not being upheld or supported by the type system.
    fn compute_score(&self, profile: &Profile<Ballot>) -> Result<Score<Self::Output>, Self::Error>;

    /// Construct a new scorer.
    fn new() -> Self;
}
