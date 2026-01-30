//! Ballot types module.
//!
//! This module contains the struct `RankingBallot`

use crate::models::candidate_id::CandidateId;

/// Ranking ballot type.
///
/// Represents a full ranking of one candidate.
pub struct RankingBallot {
    /// Candidates ranked by preference starting from 0th index (most preferable).
    votes: Vec<CandidateId>,
}

impl RankingBallot {
    /// Create a new `RankingBallot`.
    pub fn new(votes: &[usize]) -> Self {
        Self {
            votes: votes.iter().copied().map(CandidateId::new).collect(),
        }
    }
}
