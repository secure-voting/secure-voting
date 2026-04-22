//! Ballot types module.
//!
//! This module contains the struct `ApprovalBallot`

use std::{collections::HashSet, ops::Index};

use thiserror::Error;

use crate::models::{
    candidate_id::CandidateId,
    profile::{CandidateRemovalError, Profile},
};

/// Ranking ballot type.
///
/// Represents a full ranking of one candidate.
#[derive(Clone, Debug)]
pub struct ApprovalBallot {
    /// Candidates approved by each voter.
    votes: Vec<CandidateId>,
}

impl ApprovalBallot {
    /// Create a new `ApprovalBallot`.
    pub fn new(votes: &[usize]) -> Self {
        Self {
            votes: votes.iter().copied().map(CandidateId::new).collect(),
        }
    }

    /// Get an iterator over the vote values.
    pub fn iter(&self) -> core::slice::Iter<'_, CandidateId> {
        self.votes.iter()
    }

    /// Sort the candidates inside one ballot.
    fn sort(&mut self) {
        self.votes.sort();
    }

    /// Move out into an inner representation
    #[must_use]
    pub fn into_inner(self) -> Vec<CandidateId> {
        self.votes
    }
}

impl IntoIterator for ApprovalBallot {
    type Item = CandidateId;

    type IntoIter = std::vec::IntoIter<CandidateId>;

    fn into_iter(self) -> Self::IntoIter {
        self.votes.into_iter()
    }
}

impl Index<usize> for ApprovalBallot {
    type Output = CandidateId;

    fn index(&self, index: usize) -> &Self::Output {
        &self.votes[index]
    }
}

/// Profile's approval  error type.
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
    /// Returned if the ballot contains a duplicate vote.
    #[error("Candidate ID {0} was voted at least twice")]
    DoubleVote(usize),
}

impl Profile<ApprovalBallot> {
    /// Remove the candidates from the profile.
    ///
    /// Returns error if one of the to-be-removed candidates doesn't exist.
    pub(crate) fn remove_candidates(
        self,
        candidates: Vec<CandidateId>,
    ) -> Result<Self, CandidateRemovalError> {
        if let Some(&wrong_id) = candidates
            .iter()
            .find(|candidate_id| self.active_candidates.binary_search(candidate_id).is_err())
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

                new_ranking.push(rank.into_inner());
            }

            new_votes.push(ApprovalBallot::new(&new_ranking));
        }

        // Is safe, because new_votes is a non-empty
        // list of voters, as per type's invariants.
        #[allow(clippy::unwrap_used)]
        let mut first_ballot = new_votes.first().cloned().unwrap();
        first_ballot.sort();

        Ok(Self {
            votes: new_votes,
            active_candidates: first_ballot.into_inner(),
        })
    }
}

impl TryFrom<Vec<Vec<usize>>> for Profile<ApprovalBallot> {
    type Error = ProfileError;

    /// Upholds these invariants:
    ///
    /// - At least one voter
    /// - At least one candidate
    /// - Each ballot has no duplicate votes
    ///
    /// The order of candidates doesn't matter, all the candidates are 'approved'.
    /// Closer to the beginning means more preferable.
    fn try_from(value: Vec<Vec<usize>>) -> Result<Self, Self::Error> {
        if value.is_empty() {
            return Err(ProfileError::NoVoters);
        }

        if value[0].is_empty() {
            return Err(ProfileError::NoCandidates);
        }

        let mut active_candidates = HashSet::new();

        for vote in &value {
            let mut candidates = vec![0; value[0].len()];
            for &candidate in vote {
                if candidates[candidate] != 0 {
                    return Err(ProfileError::DoubleVote(candidate));
                }

                candidates[candidate] = 1;
                active_candidates.insert(candidate);
            }
        }

        Ok(Profile {
            votes: value
                .iter()
                .map(|voter_info| ApprovalBallot::new(voter_info))
                .collect(),
            active_candidates: active_candidates
                .into_iter()
                .map(CandidateId::new)
                .collect(),
        })
    }
}
