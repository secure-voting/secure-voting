use crate::types::CandidateId;

pub trait Decider {
    fn decide(scores: &[usize]) -> Vec<CandidateId>;
}
