//! Fallthrough tiebreaker implementation.

use std::convert::Infallible;

use crate::{
    profile::{CandidateId, Profile},
    tie_breaker::{RuleOutcome, TieBreaker},
};

/// Fallthrough Tie breaker.
///
/// Does nothing:
///
/// - If there is a single winner, returns [`RuleOutcome::UniqueWinner`] with a chosen winner.
/// - If there are multiple winners, returns [`RuleOutcome::MultipleWinners`] with all of the winners.
pub struct FallthroughTieBreaker;

impl TieBreaker for FallthroughTieBreaker {
    type Error = Infallible;

    fn tie_break(
        &self,
        candidates: &[CandidateId],
        _profile: &Profile,
    ) -> Result<RuleOutcome, Self::Error> {
        match candidates.len() {
            1 => Ok(RuleOutcome::UniqueWinner(candidates[0])),
            _ => Ok(RuleOutcome::MultipleWinners(candidates.to_vec())),
        }
    }
}
