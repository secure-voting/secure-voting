//! Majority Decider implementation

use std::convert::Infallible;

use crate::{decider::Decider, profile::CandidateId};

/// Majority decider.
///
/// Selects all candidates whose score is equal to the maximum score.
/// This type is a zero-sized marker implementing [`Decider`].
pub struct MajorityDecider;

impl Decider for MajorityDecider {
    type Input = Vec<usize>;
    type Error = Infallible;

    fn decide(&self, scores: &Self::Input) -> Result<Vec<CandidateId>, Self::Error> {
        let mut cur_max = 0;
        let mut winners = vec![];

        for (idx, &score) in scores.iter().enumerate() {
            if score > cur_max {
                cur_max = score;
                winners = vec![CandidateId::new(idx)];
            } else if score == cur_max {
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

        assert_eq!(vec![3], ids(MajorityDecider.decide(&scores).unwrap()));
    }

    #[test]
    fn test_several_winners() {
        let scores = vec![0, 1, 0, 1];

        assert_eq!(vec![1, 3], ids(MajorityDecider.decide(&scores).unwrap()));
    }

    #[test]
    fn test_all_winners() {
        let scores = vec![1, 1, 1, 1, 1];

        assert_eq!(
            vec![0, 1, 2, 3, 4],
            ids(MajorityDecider.decide(&scores).unwrap())
        );
    }
}
