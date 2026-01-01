use thiserror::Error;

#[derive(Debug, Error)]
pub enum ProfileError {
    #[error("Votes have different numbers of candidates")]
    DifferentVoteLengths,
    #[error("Candidate ID {0} was incorrect")]
    InvalidCandidateId(usize),
    #[error("Candidate ID {0} was voted at least twice")]
    DoubleVote(usize),
}
