//! Pairwise Matrix type implementation.
//!
//! This module defines the [`PairwiseMatrix`] type and a derived type [`CondorcetMatrix`].

pub mod condorcet;
pub mod pairwise;

pub use condorcet::CondorcetMatrix;
pub use pairwise::PairwiseMatrix;
