//! Voting profile implementation.
//!
//! A [`Profile`] represents a validated collection of voters' ballots.
//! Each ballot is a ranking of candidates.

use nutype::nutype;
use std::{collections::HashSet, ops::Index};
use thiserror::Error;

/// Strongly-typed Candidate ID.
#[nutype(derive(Debug, PartialEq, Eq, Clone, Copy, Display, Hash, PartialOrd, Ord))]
pub struct CandidateId(usize);

/// Profile type.
///
/// Wraps the votes as a newtype.
/// Upholds these invariants:
///
/// - At least one voter
/// - At least one candidate
/// - All ballots have the same length
/// - All candidates' IDs are valid
/// - Each ballot has no duplicate votes
///
/// The order of candidates in each ballot represents the preference of chosen voter.
/// Closer to the beginning means more preferable.
///
/// Only constructed through the [`TryFrom`] trait to enforce invariants.
#[derive(Debug, Clone)]
pub struct Profile {
    /// A list of ranking ballots.
    votes: Vec<Vec<CandidateId>>,
    /// Candidates that participate.
    active_candidates: Vec<CandidateId>,
}

/// Profile's error type.
///
/// Is returned upon construction using the [`TryFrom`] trait.
#[derive(Debug, Error)]
pub enum ProfileError {
    /// Returned if there are no voters in the profile
    #[error("No voters")]
    NoVoters,
    /// Returned if there are no candidates in the profile
    #[error("No candidates")]
    NoCandidates,
    /// Returned if ballots from the same profile have different lengths.
    #[error("Votes have different numbers of candidates")]
    DifferentVoteLengths,
    /// Returned if there is a candidate with an ID too big for the current length (they should be 0..len).
    #[error("Candidate ID {0} was incorrect")]
    InvalidCandidateId(usize),
    /// Returned if the ballot contains a duplicate vote.
    #[error("Candidate ID {0} was voted at least twice")]
    DoubleVote(usize),
}

/// An error returned if the candidate removed is not present in the profile.
#[derive(Error, Debug, PartialEq)]
#[error("Can't remove the candidate {0}")]
pub struct CandidateRemovalError(CandidateId);

impl Profile {
    /// Number of candidates in the current profile.
    pub fn n_candidates(&self) -> usize {
        self.votes[0].len()
    }

    /// Number of voters in the current profile.
    pub fn n_voters(&self) -> usize {
        self.votes.len()
    }

    /// Participating candidates.
    pub fn active_candidates(&self) -> &[CandidateId] {
        &self.active_candidates
    }

    /// Get candidate's position in a sorted list.
    pub fn get_candidate_id(&self, candidate: &CandidateId) -> Result<usize, usize> {
        self.active_candidates.binary_search(candidate)
    }

    /// Remove the candidates from the profile.
    pub fn remove_candidates(
        self,
        candidates: Vec<CandidateId>,
    ) -> Result<Self, CandidateRemovalError> {
        if let Some(&wrong_id) = candidates
            .iter()
            .find(|candidate_id| candidate_id.into_inner() >= self.n_candidates())
        {
            return Err(CandidateRemovalError(wrong_id));
        }

        let to_remove = candidates.into_iter().collect::<HashSet<_>>();

        let mut new_votes = Vec::with_capacity(self.n_voters());
        let n_candidates = self.n_candidates();

        let votes = self.votes;

        for voter_ranking in votes {
            let mut new_ranking = Vec::with_capacity(n_candidates - to_remove.len());

            for rank in voter_ranking {
                if to_remove.contains(&rank) {
                    continue;
                }

                new_ranking.push(rank);
            }

            new_votes.push(new_ranking);
        }

        let mut first_ballot = new_votes.first().cloned().unwrap();
        first_ballot.sort();

        Ok(Self {
            votes: new_votes,
            active_candidates: first_ballot,
        })
    }
}

impl Index<usize> for Profile {
    type Output = Vec<CandidateId>;

    fn index(&self, index: usize) -> &Self::Output {
        &self.votes[index]
    }
}

impl TryFrom<Vec<Vec<usize>>> for Profile {
    type Error = ProfileError;

    fn try_from(value: Vec<Vec<usize>>) -> Result<Self, Self::Error> {
        if value.is_empty() {
            return Err(ProfileError::NoVoters);
        }

        if value[0].is_empty() {
            return Err(ProfileError::NoCandidates);
        }

        if (1..value.len()).any(|row| value[row].len() != value[0].len()) {
            return Err(ProfileError::DifferentVoteLengths);
        }

        for vote in &value {
            let mut candidates = vec![0; value[0].len()];
            for &candidate in vote {
                if candidate >= value[0].len() {
                    return Err(ProfileError::InvalidCandidateId(candidate));
                }

                if candidates[candidate] != 0 {
                    return Err(ProfileError::DoubleVote(candidate));
                }

                candidates[candidate] = 1;
            }
        }

        Ok(Profile {
            votes: value
                .iter()
                .map(|voter_info| {
                    voter_info
                        .iter()
                        .map(|&cand_id| CandidateId::new(cand_id))
                        .collect()
                })
                .collect(),
            active_candidates: (0..value[0].len()).map(|id| CandidateId::new(id)).collect(),
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn ids(votes: Vec<Vec<CandidateId>>) -> Vec<Vec<usize>> {
        votes
            .iter()
            .map(|line| line.iter().map(|item| item.into_inner()).collect())
            .collect()
    }

    #[test]
    fn test_incorrect_no_voters() {
        let votes = vec![];

        assert!(matches!(
            TryInto::<Profile>::try_into(votes),
            Err(ProfileError::NoVoters)
        ));
    }

    #[test]
    fn test_incorrect_no_candidates() {
        let votes = vec![vec![]];

        assert!(matches!(
            TryInto::<Profile>::try_into(votes),
            Err(ProfileError::NoCandidates)
        ));
    }

    #[test]
    fn test_incorrect_different_vote_lenghts() {
        let votes = vec![vec![0, 1, 2], vec![0, 1, 2, 3]];

        assert!(matches!(
            TryInto::<Profile>::try_into(votes),
            Err(ProfileError::DifferentVoteLengths)
        ));
    }

    #[test]
    fn test_incorrect_candidate_out_of_range_edge_case() {
        let votes = vec![vec![0, 1, 3]];

        assert!(matches!(
            TryInto::<Profile>::try_into(votes),
            Err(ProfileError::InvalidCandidateId(_))
        ));
    }

    #[test]
    fn test_incorrect_candidate_out_of_range() {
        let votes = vec![vec![0, 1, 4]];

        assert!(matches!(
            TryInto::<Profile>::try_into(votes),
            Err(ProfileError::InvalidCandidateId(_))
        ));
    }

    #[test]
    fn test_incorrect_double_votes() {
        let votes = vec![vec![0, 1, 2], vec![0, 1, 0]];

        assert!(matches!(
            TryInto::<Profile>::try_into(votes),
            Err(ProfileError::DoubleVote(_))
        ));
    }

    #[test]
    fn remove_single_candidate_middle() {
        let votes = vec![vec![0, 1, 2, 3], vec![3, 2, 1, 0]];
        let profile: Profile = votes.try_into().unwrap();

        let result = profile
            .remove_candidates(vec![CandidateId::new(1)])
            .unwrap();

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

        assert_eq!(result.votes, expected_votes);
    }

    #[test]
    fn remove_multiple_candidates() {
        let votes = vec![vec![0, 1, 2, 3]];
        let profile: Profile = votes.try_into().unwrap();

        let result = profile
            .remove_candidates(vec![CandidateId::new(1), CandidateId::new(3)])
            .unwrap();

        let expected_votes = vec![vec![CandidateId::new(0), CandidateId::new(2)]];

        assert_eq!(result.votes, expected_votes);
    }

    #[test]
    fn remove_first_and_last_candidate() {
        let votes = vec![vec![0, 1, 2, 3]];
        let profile: Profile = votes.try_into().unwrap();

        let result = profile
            .remove_candidates(vec![CandidateId::new(0), CandidateId::new(3)])
            .unwrap();

        let expected_votes = vec![vec![CandidateId::new(1), CandidateId::new(2)]];

        assert_eq!(result.votes, expected_votes);
    }

    #[test]
    fn remove_all_candidates() {
        let votes = vec![vec![0, 1, 2]];
        let profile: Profile = votes.try_into().unwrap();

        let result = profile
            .remove_candidates(vec![
                CandidateId::new(0),
                CandidateId::new(1),
                CandidateId::new(2),
            ])
            .unwrap();

        assert_eq!(result.votes, vec![vec![]]);
    }

    #[test]
    fn remove_no_candidates_returns_same_profile() {
        let votes = vec![vec![0, 1, 2]];
        let profile: Profile = votes.clone().try_into().unwrap();

        let result = profile.remove_candidates(vec![]).unwrap();

        assert_eq!(ids(result.votes), votes);
    }

    #[test]
    fn remove_candidate_invalid_id_returns_error() {
        let votes = vec![vec![0, 1, 2]];
        let profile: Profile = votes.try_into().unwrap();

        let err = profile
            .remove_candidates(vec![CandidateId::new(3)])
            .unwrap_err();

        assert_eq!(err, CandidateRemovalError(CandidateId::new(3)));
    }

    #[test]
    fn remove_candidates_preserves_order() {
        let votes = vec![vec![0, 2, 1, 3], vec![3, 1, 2, 0]];
        let profile: Profile = votes.try_into().unwrap();

        let result = profile
            .remove_candidates(vec![CandidateId::new(2)])
            .unwrap();

        let expected_votes = vec![
            vec![
                CandidateId::new(0),
                CandidateId::new(1),
                CandidateId::new(3),
            ],
            vec![
                CandidateId::new(3),
                CandidateId::new(1),
                CandidateId::new(0),
            ],
        ];

        assert_eq!(result.votes, expected_votes);
    }
}
