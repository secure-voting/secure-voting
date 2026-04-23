//! Ballot types module.
//!
//! This module contains the struct `RankingBallot`

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
pub struct RankingBallot {
    /// Candidates ranked by preference starting from 0th index (most preferable).
    votes: Vec<CandidateId>,
}

impl RankingBallot {
    /// Create a new `RankingBallot`.
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

impl IntoIterator for RankingBallot {
    type Item = CandidateId;

    type IntoIter = std::vec::IntoIter<CandidateId>;

    fn into_iter(self) -> Self::IntoIter {
        self.votes.into_iter()
    }
}

impl Index<usize> for RankingBallot {
    type Output = CandidateId;

    fn index(&self, index: usize) -> &Self::Output {
        &self.votes[index]
    }
}

/// Profile's ranking error type.
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
    /// Returned if the candidate names don't match the number of them.
    #[error("There are {0} candidates' names, but {1} candidates")]
    CandidateLengthMismatch(usize, usize),
}

impl Profile<RankingBallot> {
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

            new_votes.push(RankingBallot::new(&new_ranking));
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

impl TryFrom<(Vec<Vec<usize>>, Vec<String>)> for Profile<RankingBallot> {
    type Error = ProfileError;

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
    fn try_from(value: (Vec<Vec<usize>>, Vec<String>)) -> Result<Self, Self::Error> {
        let (value, names) = value;

        if value.is_empty() {
            return Err(ProfileError::NoVoters);
        }

        if value[0].is_empty() {
            return Err(ProfileError::NoCandidates);
        }

        if (1..value.len()).any(|row| value[row].len() != value[0].len()) {
            return Err(ProfileError::DifferentVoteLengths);
        }

        let max_id = value.iter().flat_map(|v| v.iter()).copied().max().unwrap_or(0);
        if max_id >= names.len() {
            return Err(ProfileError::InvalidCandidateId(max_id));
        }

        if value[0].len() > names.len() {
            return Err(ProfileError::CandidateLengthMismatch(
                value[0].len(),
                names.len(),
            ));
        }

        // If we have more names than ballot positions, that's ok - those candidates simply weren't ranked.

        for vote in &value {
            let mut seen = vec![false; names.len()];
            for &candidate in vote {
                if seen[candidate] {
                    return Err(ProfileError::DoubleVote(candidate));
                }
                seen[candidate] = true;
            }
        }

        Ok(Profile {
            votes: value
                .iter()
                .map(|voter_info| {
                    RankingBallot::new(
                        &voter_info
                            .iter()
                            .map(|&elem| CandidateId::new(elem, names[elem].clone()))
                            .collect::<Vec<_>>(),
                    )
                })
                .collect(),
            active_candidates: (0..value[0].len())
                .zip(names.iter())
                .map(|(id, name)| CandidateId::new(id, name))
                .collect(),
        })
    }
}
