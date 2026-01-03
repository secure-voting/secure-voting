//! Condorcet Matrix type implementation.
//!
//! This module defines the [`CondorcetMatrix`] type as well as its error type [`CondorcetMatrixError`].

pub mod condorcet;
pub mod pairwise;

pub use condorcet::CondorcetMatrix;
pub use pairwise::PairwiseMatrix;
