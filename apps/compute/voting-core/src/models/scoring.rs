//! Scoring ballot module.
//!
//! This module contains the struct `ScoreBallot`

use std::{collections::HashMap, ops::Index};

use thiserror::Error;

use crate::models::{
    candidate_id::CandidateId,
    profile::{CandidateRemovalError, Profile},
    BallotData,
};

/// Scoring ballot type.
///
/// Represents a map from candidates to their scores.
#[derive(Clone, Debug)]
pub struct ScoreBallot {
    /// Scores for each candidate, stored as a map from candidate ID to score.
    scores: HashMap<CandidateId, usize>,
}

impl ScoreBallot {
    /// Create a new `ScoreBallot` from a slice of (candidate, score) pairs.
    pub fn new(scores: &[(CandidateId, usize)]) -> Self {
        Self {
            scores: scores.iter().cloned().collect(),
        }
    }

    /// Get an iterator over the (candidate, score) pairs.
    pub fn iter(&self) -> std::collections::hash_map::Iter<'_, CandidateId, usize> {
        self.scores.iter()
    }

    /// Get the score for a specific candidate.
    #[must_use]
    pub fn get_score(&self, candidate: &CandidateId) -> Option<usize> {
        self.scores.get(candidate).copied()
    }

    /// Move out into an inner representation.
    #[must_use]
    pub fn into_inner(self) -> Vec<(CandidateId, usize)> {
        self.scores.into_iter().collect()
    }
}

impl IntoIterator for ScoreBallot {
    type Item = (CandidateId, usize);

    type IntoIter = std::collections::hash_map::IntoIter<CandidateId, usize>;

    fn into_iter(self) -> Self::IntoIter {
        self.scores.into_iter()
    }
}

impl Index<CandidateId> for ScoreBallot {
    type Output = usize;

    fn index(&self, index: CandidateId) -> &Self::Output {
        &self.scores[&index]
    }
}

/// Profile's scoring error type.
///
/// Is returned upon construction using the [`TryFrom`] trait.
#[derive(Debug, Error)]
pub enum ProfileError {
    /// Returned if there are no voters in the profile.
    #[error("No voters")]
    NoVoters,
    /// Returned if there are no candidates in the profile.
    #[error("No candidates")]
    NoCandidates,
    /// Returned if a candidate has an ID too big for the current length.
    #[error("Candidate ID {0} was incorrect")]
    InvalidCandidateId(usize),
    /// Returned if the candidate names don't match the number of them.
    #[error("There are {0} candidates' names, but {1} candidates")]
    CandidateLengthMismatch(usize, usize),
}

impl Profile<ScoreBallot> {
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

        let to_remove = candidates.into_iter().collect::<std::collections::HashSet<_>>();

        let mut new_votes = Vec::with_capacity(self.n_voters());

        for voter_scores in self.votes {
            let mut new_scores = HashMap::new();
            for (candidate, score) in voter_scores.into_inner() {
                if !to_remove.contains(&candidate) {
                    new_scores.insert(candidate, score);
                }
            }
            new_votes.push(ScoreBallot { scores: new_scores });
        }

        Ok(Self {
            votes: new_votes,
            active_candidates: self
                .active_candidates
                .into_iter()
                .filter(|c| !to_remove.contains(c))
                .collect(),
        })
    }
}

impl TryFrom<(Vec<BallotData>, Vec<String>)> for Profile<ScoreBallot> {
    type Error = ProfileError;

    /// Upholds these invariants:
    ///
    /// - At least one voter
    /// - At least one candidate
    /// - All candidate IDs are valid
    ///
    /// Each ballot contains scores for candidates.
    fn try_from(value: (Vec<BallotData>, Vec<String>)) -> Result<Self, Self::Error> {
        let (ballots, names) = value;

        if ballots.is_empty() {
            return Err(ProfileError::NoVoters);
        }

        if names.is_empty() {
            return Err(ProfileError::NoCandidates);
        }

        let mut active_candidates_set = std::collections::HashSet::new();
        let mut votes = Vec::with_capacity(ballots.len());

        for ballot in &ballots {
            let BallotData::Scoring(scores) = ballot else {
                continue;
            };

            let mut ballot_scores = Vec::new();
            for &(candidate_id, score) in scores {
                if candidate_id >= names.len() {
                    return Err(ProfileError::InvalidCandidateId(candidate_id));
                }
                let candidate = CandidateId::new(candidate_id, names[candidate_id].clone());
                ballot_scores.push((candidate, score));
                active_candidates_set.insert(candidate_id);
            }
            votes.push(ScoreBallot::new(&ballot_scores));
        }

        if votes.is_empty() {
            return Err(ProfileError::NoVoters);
        }

        let active_candidates: Vec<CandidateId> = (0..names.len())
            .filter(|id| active_candidates_set.contains(id))
            .map(|id| CandidateId::new(id, names[id].clone()))
            .collect();

        Ok(Profile {
            votes,
            active_candidates,
        })
    }
}
