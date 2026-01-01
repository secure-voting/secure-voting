use std::ops::Index;

use crate::errors::ProfileError;

pub type CandidateId = usize;

#[derive(Debug, Clone)]
pub struct Profile {
    votes: Vec<Vec<usize>>,
}

impl Profile {
    pub fn n_candidates(&self) -> usize {
        self.votes[0].len()
    }

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
