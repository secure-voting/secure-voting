//! Voting profile implementation.
//!
//! A [`Profile`] represents a validated collection of voters' ballots.
//! Each ballot is a ranking of candidates.

use nutype::nutype;
use std::ops::Index;
use thiserror::Error;

/// Strongly-typed Candidate ID.
#[nutype(derive(Debug, PartialEq, Eq, Clone, Copy, Display))]
pub struct CandidateId(usize);

/// Profile type.
///
/// Wraps the votes as a newtype.
/// Upholds these invariants:
///
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
    votes: Vec<Vec<usize>>,
}

/// Profile's error type.
///
/// Is returned upon construction using the [`TryFrom`] trait.
#[derive(Debug, Error)]
pub enum ProfileError {
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

impl Profile {
    /// Number of candidates in the current profile.
    pub fn n_candidates(&self) -> usize {
        self.votes[0].len()
    }

    /// Number of voters in the current profile.
    pub fn n_voters(&self) -> usize {
        self.votes.len()
    }
}

impl Index<usize> for Profile {
    type Output = Vec<usize>;

    fn index(&self, index: usize) -> &Self::Output {
        &self.votes[index]
    }
}

impl TryFrom<Vec<Vec<usize>>> for Profile {
    type Error = ProfileError;

    fn try_from(value: Vec<Vec<usize>>) -> Result<Self, Self::Error> {
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

        Ok(Profile { votes: value })
    }
}

#[cfg(test)]
mod tests {
    use super::*;

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
}
