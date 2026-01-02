//! Fallthrough tiebreaker implementation.

use std::convert::Infallible;

use crate::{
    profile::{CandidateId, Profile},
    tie_breaker::{RuleOutcome, TieBreaker},
};

pub struct FallthroughTieBreaker;

impl TieBreaker for FallthroughTieBreaker {
    type Error = Infallible;

    fn tie_break(
        &self,
        candidates: &[CandidateId],
        _profile: &Profile,
    ) -> Result<RuleOutcome, Self::Error> {
        match candidates.len() {
            1 => Ok(RuleOutcome::Winner(candidates[0])),
            _ => Ok(RuleOutcome::Undecided),
        }
    }
}
