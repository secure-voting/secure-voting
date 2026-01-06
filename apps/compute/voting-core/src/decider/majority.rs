//! Majority Decider implementation

use std::{convert::Infallible, marker::PhantomData};

use crate::{decider::Decider, profile::CandidateId};

/// Majority decider.
///
/// Selects all candidates whose score is equal to the maximum score.
/// This type is a zero-sized marker implementing [`Decider`].
pub struct MajorityDecider<T> {
    _marker: PhantomData<T>,
}

impl<T> MajorityDecider<T> {
    pub fn new() -> Self {
        Self {
            _marker: PhantomData::<T>,
        }
    }
}

impl<T> Decider for MajorityDecider<T>
where
    T: PartialOrd + Default + Copy,
{
    type Input = Vec<T>;
    type Error = Infallible;

    fn decide(&self, scores: &Self::Input) -> Result<Vec<CandidateId>, Self::Error> {
        let mut cur_max = None;
        let mut winners = vec![];

        for (idx, &score) in scores.iter().enumerate() {
            if cur_max.is_none() {
                cur_max = Some(score);
                winners = vec![CandidateId::new(idx)];
            } else if let Some(cur_max_inner) = cur_max
                && cur_max_inner < score
            {
                cur_max = Some(score);
                winners = vec![CandidateId::new(idx)];
            } else if Some(score) == cur_max {
                winners.push(CandidateId::new(idx));
            }
        }

        Ok(winners)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn ids(v: Vec<CandidateId>) -> Vec<usize> {
        v.into_iter().map(|x| x.into_inner()).collect()
    }

    #[test]
    fn test_one_winner() {
        let scores = vec![0, 1, 0, 2];

        assert_eq!(
            vec![3],
            ids(MajorityDecider::new().decide(&scores).unwrap())
        );
    }

    #[test]
    fn test_several_winners() {
        let scores = vec![0, 1, 0, 1];

        assert_eq!(
            vec![1, 3],
            ids(MajorityDecider::new().decide(&scores).unwrap())
        );
    }

    #[test]
    fn test_all_winners() {
        let scores = vec![1, 1, 1, 1, 1];

        assert_eq!(
            vec![0, 1, 2, 3, 4],
            ids(MajorityDecider::new().decide(&scores).unwrap())
        );
    }
}
