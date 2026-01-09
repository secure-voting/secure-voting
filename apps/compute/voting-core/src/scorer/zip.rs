//! Zip scorer module.

use thiserror::Error;

use crate::{
    prelude::Profile,
    scorer::{Score, Scorer},
};

/// Zip scorer.
///
/// Runs two scorer and returns a tuple of their computed scores.
pub struct ZipScorer<S1, S2> {
    scorer1: S1,
    scorer2: S2,
}

impl<S1, S2> ZipScorer<S1, S2> {
    /// Construct a ZipScorer from 2 scorers.
    pub fn new(scorer1: S1, scorer2: S2) -> Self {
        Self { scorer1, scorer2 }
    }
}

/// Zip scorer error type.
#[derive(Error, Debug)]
pub enum ZipScorerError<SE1, SE2> {
    /// First scorer error.
    #[error(transparent)]
    FirstScorerError(SE1),
    /// Second scorer error.
    #[error(transparent)]
    SecondScorerError(SE2),
}

impl<T1, T2, S1: Scorer<Output = T1>, S2: Scorer<Output = T2>> Scorer for ZipScorer<S1, S2> {
    type Output = (T1, T2);

    type Error = ZipScorerError<S1::Error, S2::Error>;

    fn compute_score(&self, profile: &Profile) -> Result<Score<Self::Output>, Self::Error> {
        let score1 = self
            .scorer1
            .compute_score(profile)
            .map_err(ZipScorerError::FirstScorerError)?;
        let score2 = self
            .scorer2
            .compute_score(profile)
            .map_err(ZipScorerError::SecondScorerError)?;

        Ok(Score::new(
            (score1.consume_score(), score2.consume_score()),
            profile.active_candidates(),
        ))
    }

    fn new() -> Self {
        Self {
            scorer1: S1::new(),
            scorer2: S2::new(),
        }
    }
}
