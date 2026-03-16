//! Majority Decider implementation

use std::{convert::Infallible, marker::PhantomData};

use crate::{decider::Decider, models::candidate_id::CandidateId, scorer::Score};

/// Majority decider.
///
/// Selects all candidates whose score is equal to the maximum score.
/// This type is a zero-sized marker implementing [`Decider`].
#[derive(Default, Debug, Clone, Copy)]
pub struct MaxScoreDecider<T> {
    /// `PhantomData` type marker to allow generics inside this struct.
    _marker: PhantomData<T>,
}

impl<T> Decider for MaxScoreDecider<T>
where
    T: PartialOrd + Default + Copy,
{
    type Input = Vec<T>;
    type Error = Infallible;

    fn decide(&self, scores: &Score<Self::Input>) -> Result<Vec<CandidateId>, Self::Error> {
        let mut cur_max = None;
        let mut winners = vec![];

        for (score, &cand_id) in scores.iter() {
            if cur_max.is_none() {
                cur_max = Some(score);
                winners = vec![cand_id];
            } else if let Some(cur_max_inner) = cur_max
                && cur_max_inner < score
            {
                cur_max = Some(score);
                winners = vec![cand_id];
            } else if Some(score) == cur_max {
                winners.push(cand_id);
            }
        }

        Ok(winners)
    }

    fn new() -> Self {
        Self {
            _marker: PhantomData,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn ids(v: Vec<CandidateId>) -> Vec<usize> {
        v.into_iter().map(CandidateId::into_inner).collect()
    }

    #[test]
    fn one_winner() {
        let scores = Score::new(
            vec![0, 1, 0, 2],
            &[
                CandidateId::new(1),
                CandidateId::new(2),
                CandidateId::new(9),
                CandidateId::new(0),
            ],
        );

        assert_eq!(
            vec![0],
            ids(MaxScoreDecider::new().decide(&scores).unwrap())
        );
    }

    #[test]
    fn several_winners() {
        let scores = Score::new(
            vec![0, 1, 0, 1],
            &[
                CandidateId::new(1),
                CandidateId::new(2),
                CandidateId::new(9),
                CandidateId::new(0),
            ],
        );

        assert_eq!(
            vec![2, 0],
            ids(MaxScoreDecider::new().decide(&scores).unwrap())
        );
    }

    #[test]
    fn all_winners() {
        let scores = Score::new(
            vec![1, 1, 1, 1, 1],
            &[
                CandidateId::new(42),
                CandidateId::new(1),
                CandidateId::new(2),
                CandidateId::new(9),
                CandidateId::new(0),
            ],
        );

        assert_eq!(
            vec![42, 1, 2, 9, 0],
            ids(MaxScoreDecider::new().decide(&scores).unwrap())
        );
    }
}
