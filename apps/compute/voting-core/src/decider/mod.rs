use crate::types::CandidateId;

pub mod plurality;

pub trait Decider {
    fn decide(scores: &[usize]) -> Vec<CandidateId>;
}
