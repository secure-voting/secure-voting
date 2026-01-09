//! Minority Decider implementation

use std::{convert::Infallible, marker::PhantomData};

use crate::{decider::Decider, profile::CandidateId, scorer::Score};

/// Minority decider.
///
/// Selects all candidates whose score is equal to the minimum score.
/// This type is a zero-sized marker implementing [`Decider`].
#[derive(Default)]
pub struct MinorityDecider<T> {
    /// PhantomData type marker to allow generics inside this struct.
    _marker: PhantomData<T>,
}

impl<T> Decider for MinorityDecider<T>
where
    T: PartialOrd + Default + Copy,
{
    type Input = Vec<T>;
    type Error = Infallible;

    fn decide(&self, scores: &Score<Self::Input>) -> Result<Vec<CandidateId>, Self::Error> {
        let mut cur_min = None;
        let mut winners = vec![];

        for (score, &cand_id) in scores.iter() {
            if cur_min.is_none() {
                cur_min = Some(score);
                winners = vec![cand_id];
            } else if let Some(cur_min_inner) = cur_min
                && cur_min_inner > score
            {
                cur_min = Some(score);
                winners = vec![cand_id];
            } else if Some(score) == cur_min {
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
        v.into_iter().map(|x| x.into_inner()).collect()
    }

    #[test]
    fn test_one_winner() {
        let scores = Score::new(
            vec![2, 1, 2, 0],
            &vec![
                CandidateId::new(1),
                CandidateId::new(2),
                CandidateId::new(9),
                CandidateId::new(0),
            ],
        );

        assert_eq!(
            vec![0],
            ids(MinorityDecider::new().decide(&scores).unwrap())
        );
    }

    #[test]
    fn test_several_winners() {
        let scores = Score::new(
            vec![3, 2, 3, 2],
            &vec![
                CandidateId::new(1),
                CandidateId::new(2),
                CandidateId::new(9),
                CandidateId::new(0),
            ],
        );

        assert_eq!(
            vec![2, 0],
            ids(MinorityDecider::new().decide(&scores).unwrap())
        );
    }

    #[test]
    fn test_all_winners() {
        let scores = Score::new(
            vec![1, 1, 1, 1, 1],
            &vec![
                CandidateId::new(10),
                CandidateId::new(1),
                CandidateId::new(2),
                CandidateId::new(9),
                CandidateId::new(0),
            ],
        );

        assert_eq!(
            vec![10, 1, 2, 9, 0],
            ids(MinorityDecider::new().decide(&scores).unwrap())
        );
    }
}
