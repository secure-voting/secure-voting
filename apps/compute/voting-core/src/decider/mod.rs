//! Functionality for choosing a set of winners from computed scores.
//!
//! This module defines the [`Decider`] trait and provides pre-packaged decider implementations.

use std::fmt::Debug;

use crate::{models::candidate_id::CandidateId, scorer::Score};

pub mod condorcet;
pub mod maxscore;
pub mod minscore;
pub mod threshold;

/// Chooses a winner or a set of winners from computed scores.
pub trait Decider {
    /// Input produced by a [`super::scorer::Scorer`] and consumed by this decider.
    type Input;

    /// Error returned when a decision cannot be made.
    ///
    /// Use [`std::convert::Infallible`] if this decider cannot fail.
    type Error: Debug;

    /// Decides the winner or a set of winners from the scores provided.
    ///
    /// # Errors
    ///
    /// Returns an error of the implementation of a decider cannot compute
    /// the decision step infallibly, usually caused by an invariant
    /// inherent to the decision rule not expressed in the type system.
    fn decide(&self, scores: &Score<Self::Input>) -> Result<Vec<CandidateId>, Self::Error>;

    /// Construct a new Decider instance.
    fn new() -> Self;
}
