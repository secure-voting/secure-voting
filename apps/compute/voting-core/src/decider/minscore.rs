//! Minority Decider implementation

use std::{convert::Infallible, marker::PhantomData};

use crate::{decider::Decider, models::candidate_id::CandidateId, scorer::Score};

/// Minority decider.
///
/// Selects all candidates whose score is equal to the minimum score.
/// This type is a zero-sized marker implementing [`Decider`].
#[derive(Default, Debug, Clone, Copy)]
pub struct MinScoreDecider<T> {
    /// `PhantomData` type marker to allow generics inside this struct.
    _marker: PhantomData<T>,
}

impl<T> Decider for MinScoreDecider<T>
where
    T: PartialOrd + Default + Copy,
{
    type Input = Vec<T>;
    type Error = Infallible;

    fn decide(&self, scores: &Score<Self::Input>) -> Result<Vec<CandidateId>, Self::Error> {
        let mut cur_min = None;
        let mut winners = vec![];

        for (score, cand_id) in scores.iter() {
            if cur_min.is_none() {
                cur_min = Some(score);
                winners = vec![cand_id.clone()];
            } else if let Some(cur_min_inner) = cur_min
                && cur_min_inner > score
            {
                cur_min = Some(score);
                winners = vec![cand_id.clone()];
            } else if Some(score) == cur_min {
                winners.push(cand_id.clone());
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

    fn cid(id: usize) -> CandidateId {
        CandidateId::new(id, format!("C{id}"))
    }

    fn ids(v: Vec<CandidateId>) -> Vec<usize> {
        v.iter().map(CandidateId::get_id).collect()
    }

    #[test]
    fn one_winner() {
        let scores = Score::new(vec![2, 1, 2, 0], &[cid(1), cid(2), cid(9), cid(0)]);

        assert_eq!(
            vec![0],
            ids(MinScoreDecider::new().decide(&scores).unwrap())
        );
    }

    #[test]
    fn several_winners() {
        let scores = Score::new(vec![3, 2, 3, 2], &[cid(1), cid(2), cid(9), cid(0)]);

        assert_eq!(
            vec![2, 0],
            ids(MinScoreDecider::new().decide(&scores).unwrap())
        );
    }

    #[test]
    fn all_winners() {
        let scores = Score::new(
            vec![1, 1, 1, 1, 1],
            &[cid(10), cid(1), cid(2), cid(9), cid(0)],
        );

        assert_eq!(
            vec![10, 1, 2, 9, 0],
            ids(MinScoreDecider::new().decide(&scores).unwrap())
        );
    }
}
