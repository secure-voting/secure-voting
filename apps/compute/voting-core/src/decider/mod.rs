use crate::profile::CandidateId;

pub mod plurality;

pub trait Decider {
    type Input;
    type Error;

    fn decide(&self, scores: &Self::Input) -> Result<Vec<CandidateId>, Self::Error>;
}
