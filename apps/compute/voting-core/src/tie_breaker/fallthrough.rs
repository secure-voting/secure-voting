//! Fallthrough tiebreaker implementation.

use std::convert::Infallible;

use crate::{
    models::{candidate_id::CandidateId, profile::Profile},
    tie_breaker::{RuleOutcome, TieBreaker},
};

/// Fallthrough Tie breaker.
///
/// Does nothing:
///
/// - If there is a single winner, returns [`RuleOutcome::UniqueWinner`] with a chosen winner.
/// - If there are multiple winners, returns [`RuleOutcome::MultipleWinners`] with all of the winners.
#[derive(Debug, Clone, Copy)]
pub struct FallthroughTieBreaker;

impl<Ballot> TieBreaker<Ballot> for FallthroughTieBreaker {
    type Error = Infallible;

    fn tie_break(
        &self,
        candidates: &[CandidateId],
        _profile: &Profile<Ballot>,
    ) -> Result<RuleOutcome, Self::Error> {
        match candidates.len() {
            1 => Ok(RuleOutcome::UniqueWinner(candidates[0])),
            _ => Ok(RuleOutcome::MultipleWinners(candidates.to_vec())),
        }
    }

    fn new() -> Self {
        Self
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn unique_winner_fallthrough() {
        let fake_profile = (vec![vec![0]])
            .try_into()
            .expect("Profile is constructed incorrectly, revise test example.");
        let candidates = vec![CandidateId::new(0)];

        assert_eq!(
            RuleOutcome::UniqueWinner(CandidateId::new(0)),
            FallthroughTieBreaker
                .tie_break(&candidates, &fake_profile)
                .unwrap()
        );
    }

    #[test]
    fn multiple_winner_fallthrough() {
        let fake_profile = (vec![vec![0]])
            .try_into()
            .expect("Profile is constructed incorrectly, revise test example.");
        let candidates = vec![CandidateId::new(0), CandidateId::new(42)];

        assert_eq!(
            RuleOutcome::MultipleWinners(vec![CandidateId::new(0), CandidateId::new(42)]),
            FallthroughTieBreaker
                .tie_break(&candidates, &fake_profile)
                .unwrap()
        );
    }
}
