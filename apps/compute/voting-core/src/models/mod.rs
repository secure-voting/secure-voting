//! A module containing commonly used voting types.
//!
//! This module contains submodules `approval`, `candidate_id`, `profile`, `ranking`, and `scoring`.

pub mod approval;
pub mod candidate_id;
pub mod profile;
pub mod ranking;
pub mod scoring;

use candidate_id::CandidateId;

/// Represents ballot data in different formats.
///
/// This enum allows different ballot types to share a common input format
/// when constructing a [`Profile`].
#[derive(Debug, Clone)]
pub enum BallotData {
    /// Simple ballot: list of candidate IDs (ranking or approval).
    Simple(Vec<CandidateId>),
    /// Scoring ballot: list of (candidate ID, score) pairs.
    Scoring(Vec<(CandidateId, usize)>),
}
