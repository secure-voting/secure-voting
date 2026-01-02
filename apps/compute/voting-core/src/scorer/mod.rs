use crate::profile::Profile;

pub mod approval;
pub mod plurality;

pub trait Scorer {
    type Output;
    type Error;

    fn compute_score(&self, profile: &Profile) -> Result<Self::Output, Self::Error>;
}
