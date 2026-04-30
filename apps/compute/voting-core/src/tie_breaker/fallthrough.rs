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
            1 => Ok(RuleOutcome::UniqueWinner(candidates[0].clone())),
            _ => Ok(RuleOutcome::MultipleWinners(candidates.to_vec())),
        }
    }

    fn new() -> Self {
        Self
    }
}

#[cfg(test)]
mod tests {
    use crate::models::ranking::RankingBallot;
    use crate::models::BallotData;

    use super::*;

    fn fake_profile() -> Profile<RankingBallot> {
        let ballots: Vec<BallotData> = vec![BallotData::Simple(vec![0])];
        Profile::try_from((ballots, vec!["A".into()]))
            .expect("Profile is constructed incorrectly, revise test example.")
    }

    #[test]
    fn unique_winner_fallthrough() {
        let fake_profile = fake_profile();

        let candidates = vec![CandidateId::new(0, "A")];

        assert_eq!(
            RuleOutcome::UniqueWinner(CandidateId::new(0, "A")),
            FallthroughTieBreaker
                .tie_break(&candidates, &fake_profile)
                .unwrap()
        );
    }
    #[test]
    fn multiple_winner_fallthrough() {
        let fake_profile = fake_profile();

        let candidates = vec![CandidateId::new(0, "A"), CandidateId::new(42, "X")];

        assert_eq!(
            RuleOutcome::MultipleWinners(
                vec![CandidateId::new(0, "A"), CandidateId::new(42, "X"),]
            ),
            FallthroughTieBreaker
                .tie_break(&candidates, &fake_profile)
                .unwrap()
        );
    }
}
