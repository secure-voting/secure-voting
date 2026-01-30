//! Ballot types module.
//!
//! This module contains the struct `RankingBallot`

use crate::models::candidate_id::CandidateId;

/// Ranking ballot type.
///
/// Represents a full ranking of one candidate.
struct RankingBallot {
    votes: Vec<CandidateId>,
}
