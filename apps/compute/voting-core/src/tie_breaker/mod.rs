use crate::types::{CandidateId, Profile};

pub trait TieBreaker {
    fn tie_break(candidates: &[CandidateId], profile: &Profile) -> CandidateId;
}
