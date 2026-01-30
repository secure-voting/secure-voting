//! Ballot types module.
//!
//! This module contains the struct `RankingBallot`

use crate::models::profile::CandidateId;

/// Ranking ballot type.
///
/// Represents a full ranking of one candidate.
struct RankingBallot {
    votes: Vec<CandidateId>,
}
