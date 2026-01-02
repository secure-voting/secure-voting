//! Functionality for choosing a set of winners from computed scores.
//!
//! This module defines the [`Decider`] trait and provides pre-packaged decider implementations.

use std::fmt::Debug;

use crate::profile::CandidateId;

pub mod plurality;

/// Chooses a winner or a set of winners from computed scores.
pub trait Decider {
    /// Input produced by a [`Scorer`] and consumed by this decider.
    type Input;

    /// Error returned when a decision cannot be made.
    ///
    /// Use [`std::convert::Infallible`] if this decider cannot fail.
    type Error: Debug;

    /// Decides the winner or a set of winners from the scores provided.
    fn decide(&self, scores: &Self::Input) -> Result<Vec<CandidateId>, Self::Error>;
}
