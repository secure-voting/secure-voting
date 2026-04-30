//! Ballot types module.
//!
//! This module contains the struct `ApprovalBallot`

use std::{collections::HashSet, ops::Index};

use thiserror::Error;

use crate::models::{
    BallotData,
    candidate_id::CandidateId,
    profile::{CandidateRemovalError, Profile},
};

/// Approval ballot type.
///
/// Represents a set of approved candidates.
#[derive(Clone, Debug)]
pub struct ApprovalBallot {
    /// Candidates approved by each voter.
    votes: Vec<CandidateId>,
}

impl ApprovalBallot {
    /// Create a new `ApprovalBallot`.
    #[must_use]
    pub fn new(votes: &[CandidateId]) -> Self {
        Self {
            votes: votes.to_vec(),
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
        if let Some(wrong_id) = candidates
            .iter()
            .find(|candidate_id| self.active_candidates.binary_search(candidate_id).is_err())
        {
            return Err(CandidateRemovalError(wrong_id.clone()));
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

impl TryFrom<(Vec<BallotData>, Vec<String>)> for Profile<ApprovalBallot> {
    type Error = ProfileError;

    /// Upholds these invariants:
    ///
    /// - At least one voter
    /// - At least one candidate
    /// - Each ballot has no duplicate votes
    ///
    /// The order of candidates doesn't matter, all the candidates are 'approved'.
    /// Closer to the beginning means more preferable.
    fn try_from(value: (Vec<BallotData>, Vec<String>)) -> Result<Self, Self::Error> {
        let (ballots, names) = value;

        if ballots.is_empty() {
            return Err(ProfileError::NoVoters);
        }

        let mut value: Vec<Vec<CandidateId>> = Vec::with_capacity(ballots.len());
        for ballot in &ballots {
            let BallotData::Simple(votes) = ballot else {
                continue;
            };
            if votes.is_empty() {
                return Err(ProfileError::NoCandidates);
            }
            value.push(votes.clone());
        }

        if value.is_empty() || value[0].is_empty() {
            return Err(ProfileError::NoCandidates);
        }

        let mut active_candidates = HashSet::new();

        for vote in &value {
            let mut seen = HashSet::new();
            for candidate in vote {
                if !seen.insert(candidate.clone()) {
                    return Err(ProfileError::DoubleVote(candidate.get_id()));
                }
                active_candidates.insert(candidate.clone());
            }
        }

        Ok(Profile {
            votes: value
                .iter()
                .map(|voter_info| ApprovalBallot::new(voter_info))
                .collect(),
            active_candidates: (0..value[0].len())
                .zip(names.iter())
                .map(|(id, name)| CandidateId::new(id, name))
                .collect(),
        })
    }
}
