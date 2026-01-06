//! Scoring rule applied to a voting profile prior to winner selection.
//!
//! This module defines a [`Scorer`] trait and provides pre-packaged implementations.

use std::fmt::Debug;

use crate::profile::Profile;

pub mod anti_plurality;
pub mod approval;
pub mod borda;
pub mod condorcet;
pub mod copeland;
pub mod minmax;
pub mod plurality;
pub mod simpson;

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
    fn compute_score(&self, profile: &Profile) -> Result<Self::Output, Self::Error>;
}
