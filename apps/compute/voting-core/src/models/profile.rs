//! Voting profile implementation.
//!
//! A [`Profile`] represents a validated collection of voters' ballots.
//! Each ballot is a ranking of candidates.

use std::ops::Index;
use thiserror::Error;

use crate::models::candidate_id::CandidateId;

/// Profile type.
///
/// Wraps the votes as a newtype.
///
/// Only constructed through the [`TryFrom`] trait to enforce invariants.
#[derive(Debug, Clone)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct Profile<Ballot> {
    /// A list of ballots.
    pub(crate) votes: Vec<Ballot>,
    /// Candidates that participate.
    pub(crate) active_candidates: Vec<CandidateId>,
}

/// An error returned if the candidate removed is not present in the profile.
#[derive(Error, Debug, PartialEq)]
#[error("Can't remove the candidate {0}")]
pub struct CandidateRemovalError(pub CandidateId);

impl<T> Profile<T> {
    /// Number of candidates in the current profile.
    #[must_use]
    pub fn n_candidates(&self) -> usize {
        self.active_candidates.len()
    }

    /// Number of voters in the current profile.
    #[must_use]
    pub fn n_voters(&self) -> usize {
        self.votes.len()
    }

    /// Participating candidates.
    #[must_use]
    pub fn active_candidates(&self) -> &[CandidateId] {
        &self.active_candidates
    }

    /// Get candidate's position in a sorted list.
    #[must_use]
    pub fn index_of(&self, candidate: &CandidateId) -> Option<usize> {
        self.active_candidates.binary_search(candidate).ok()
    }
}

impl<T> Index<usize> for Profile<T> {
    type Output = T;

    fn index(&self, index: usize) -> &Self::Output {
        &self.votes[index]
    }
}

#[cfg(test)]
mod tests {
    use crate::models::ranking::{ProfileError, RankingBallot};

    use super::*;

    fn ids(votes: &[RankingBallot]) -> Vec<Vec<usize>> {
        votes
            .iter()
            .map(|line| line.iter().map(|item| item.into_inner()).collect())
            .collect()
    }

    #[test]
    fn incorrect_no_voters() {
        let votes = vec![];

        assert!(matches!(
            TryInto::<Profile<RankingBallot>>::try_into(votes),
            Err(ProfileError::NoVoters)
        ));
    }

    #[test]
    fn incorrect_no_candidates() {
        let votes = vec![vec![]];

        assert!(matches!(
            TryInto::<Profile<RankingBallot>>::try_into(votes),
            Err(ProfileError::NoCandidates)
        ));
    }

    #[test]
    fn incorrect_different_vote_lenghts() {
        let votes = vec![vec![0, 1, 2], vec![0, 1, 2, 3]];

        assert!(matches!(
            TryInto::<Profile<RankingBallot>>::try_into(votes),
            Err(ProfileError::DifferentVoteLengths)
        ));
    }

    #[test]
    fn incorrect_candidate_out_of_range_edge_case() {
        let votes = vec![vec![0, 1, 3]];

        assert!(matches!(
            TryInto::<Profile<RankingBallot>>::try_into(votes),
            Err(ProfileError::InvalidCandidateId(_))
        ));
    }

    #[test]
    fn incorrect_candidate_out_of_range() {
        let votes = vec![vec![0, 1, 4]];

        assert!(matches!(
            TryInto::<Profile<RankingBallot>>::try_into(votes),
            Err(ProfileError::InvalidCandidateId(_))
        ));
    }

    #[test]
    fn incorrect_double_votes() {
        let votes = vec![vec![0, 1, 2], vec![0, 1, 0]];

        assert!(matches!(
            TryInto::<Profile<RankingBallot>>::try_into(votes),
            Err(ProfileError::DoubleVote(_))
        ));
    }

    #[test]
    fn remove_single_candidate_middle() {
        let votes = vec![vec![0, 1, 2, 3], vec![3, 2, 1, 0]];
        let profile: Profile<RankingBallot> = votes
            .try_into()
            .expect("Profile is constructed incorrectly, revise test example.");
        let result = profile
            .remove_candidates(vec![CandidateId::new(1)])
            .expect("Chosen candidate couldn't be removed from the given profile");

        let expected_votes = vec![
            vec![
                CandidateId::new(0),
                CandidateId::new(2),
                CandidateId::new(3),
            ],
            vec![
                CandidateId::new(3),
                CandidateId::new(2),
                CandidateId::new(0),
            ],
        ];

        assert_eq!(
            result
                .votes
                .into_iter()
                .map(RankingBallot::into_inner)
                .collect::<Vec<_>>(),
            expected_votes
        );
        assert_eq!(
            result.active_candidates,
            vec![
                CandidateId::new(0),
                CandidateId::new(2),
                CandidateId::new(3)
            ]
        );
    }

    #[test]
    fn remove_multiple_candidates() {
        let votes = vec![vec![0, 1, 2, 3]];
        let profile: Profile<RankingBallot> = votes
            .try_into()
            .expect("Profile is constructed incorrectly, revise test example.");

        let result = profile
            .remove_candidates(vec![CandidateId::new(1), CandidateId::new(3)])
            .expect("Chosen candidate couldn't be removed from the given profile");

        let expected_votes = vec![vec![CandidateId::new(0), CandidateId::new(2)]];

        assert_eq!(
            result
                .votes
                .into_iter()
                .map(RankingBallot::into_inner)
                .collect::<Vec<_>>(),
            expected_votes
        );
        assert_eq!(
            result.active_candidates,
            vec![CandidateId::new(0), CandidateId::new(2)]
        );
    }

    #[test]
    fn remove_first_and_last_candidate() {
        let votes = vec![vec![0, 1, 2, 3]];
        let profile: Profile<RankingBallot> = votes
            .try_into()
            .expect("Profile is constructed incorrectly, revise test example.");

        let result = profile
            .remove_candidates(vec![CandidateId::new(0), CandidateId::new(3)])
            .expect("Chosen candidate couldn't be removed from the given profile");

        let expected_votes = vec![vec![CandidateId::new(1), CandidateId::new(2)]];

        assert_eq!(
            result
                .votes
                .into_iter()
                .map(RankingBallot::into_inner)
                .collect::<Vec<_>>(),
            expected_votes
        );
        assert_eq!(
            result.active_candidates,
            vec![CandidateId::new(1), CandidateId::new(2)]
        );
    }

    #[test]
    fn remove_all_candidates() {
        let votes = vec![vec![0, 1, 2]];
        let profile: Profile<RankingBallot> = votes
            .try_into()
            .expect("Profile is constructed incorrectly, revise test example.");

        let result = profile
            .remove_candidates(vec![
                CandidateId::new(0),
                CandidateId::new(1),
                CandidateId::new(2),
            ])
            .expect("Chosen candidate couldn't be removed from the given profile");

        assert_eq!(
            result
                .votes
                .into_iter()
                .map(RankingBallot::into_inner)
                .collect::<Vec<_>>(),
            vec![vec![]]
        );
        assert_eq!(result.active_candidates, vec![]);
    }

    #[test]
    fn remove_no_candidates_returns_same_profile() {
        let votes = vec![vec![0, 1, 2]];
        let profile: Profile<RankingBallot> = votes
            .clone()
            .try_into()
            .expect("Profile is constructed incorrectly, revise test example.");

        let result = profile
            .remove_candidates(vec![])
            .expect("Chosen candidate couldn't be removed from the given profile");

        assert_eq!(ids(&result.votes), votes);
        assert_eq!(
            result.active_candidates,
            vec![
                CandidateId::new(0),
                CandidateId::new(1),
                CandidateId::new(2)
            ]
        );
    }

    #[test]
    fn remove_candidate_invalid_id_returns_error() {
        let votes = vec![vec![0, 1, 2]];
        let profile: Profile<RankingBallot> = votes
            .try_into()
            .expect("Profile is constructed incorrectly, revise test example.");

        let err = profile
            .remove_candidates(vec![CandidateId::new(3)])
            .expect_err("This candidate is incorrect, can't remove without error");

        assert_eq!(err, CandidateRemovalError(CandidateId::new(3)));
    }

    #[test]
    fn remove_candidates_preserves_order_multiple_removals() {
        let votes = vec![vec![0, 2, 1, 3], vec![3, 1, 2, 0]];
        let profile: Profile<RankingBallot> = votes
            .try_into()
            .expect("Profile is constructed incorrectly, revise test example.");

        let profile = profile
            .remove_candidates(vec![CandidateId::new(2)])
            .expect("Chosen candidate couldn't be removed from the given profile");
        let result = profile
            .remove_candidates(vec![CandidateId::new(0)])
            .expect("Chosen candidate couldn't be removed from the given profile");

        let expected_votes = vec![
            vec![CandidateId::new(1), CandidateId::new(3)],
            vec![CandidateId::new(3), CandidateId::new(1)],
        ];

        assert_eq!(
            result
                .votes
                .into_iter()
                .map(RankingBallot::into_inner)
                .collect::<Vec<_>>(),
            expected_votes
        );
        assert_eq!(
            result.active_candidates,
            vec![CandidateId::new(1), CandidateId::new(3),]
        );
    }
}
