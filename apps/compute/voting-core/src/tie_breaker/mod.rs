use crate::profile::{CandidateId, Profile};

pub trait TieBreaker {
    type Error;

    fn tie_break(
        &self,
        candidates: &[CandidateId],
        profile: &Profile,
    ) -> Result<CandidateId, Self::Error>;
}
