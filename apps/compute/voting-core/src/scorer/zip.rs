//! Zip scorer module.

use std::marker::PhantomData;

use thiserror::Error;

use crate::{
    prelude::Profile,
    scorer::{Score, Scorer},
};

/// Zip scorer.
///
/// Runs two scorer and returns a tuple of their computed scores.
#[derive(Debug, Clone, Copy)]
pub struct ZipScorer<S1, S2, Ballot> {
    /// The first scorer.
    scorer1: S1,
    /// The second scorer.
    scorer2: S2,
    /// Ballot type marker.
    _ballot_type: PhantomData<Ballot>,
}

impl<S1, S2, Ballot> ZipScorer<S1, S2, Ballot> {
    /// Construct a `ZipScorer` from 2 scorers.
    pub fn new(scorer1: S1, scorer2: S2) -> Self {
        Self {
            scorer1,
            scorer2,
            _ballot_type: PhantomData,
        }
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

impl<
    T1: Clone + PartialOrd + Ord,
    T2: Clone + PartialOrd + Ord,
    S1: Scorer<Ballot, Output = T1>,
    S2: Scorer<Ballot, Output = T2>,
    Ballot,
> Scorer<Ballot> for ZipScorer<S1, S2, Ballot>
{
    type Output = (T1, T2);

    type Error = ZipScorerError<S1::Error, S2::Error>;

    fn compute_score(&self, profile: &Profile<Ballot>) -> Result<Score<Self::Output>, Self::Error> {
        let score1 = self
            .scorer1
            .compute_score(profile)
            .map_err(ZipScorerError::FirstScorerError)?;
        let score2 = self
            .scorer2
            .compute_score(profile)
            .map_err(ZipScorerError::SecondScorerError)?;

        Ok(Score::new(
            score1
                .consume_score()
                .into_iter()
                .zip(score2.consume_score().into_iter())
                .collect(),
            profile.active_candidates(),
        ))
    }

    fn new() -> Self {
        Self {
            scorer1: S1::new(),
            scorer2: S2::new(),
            _ballot_type: PhantomData,
        }
    }
}
