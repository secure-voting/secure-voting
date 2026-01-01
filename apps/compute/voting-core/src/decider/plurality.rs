use crate::{decider::Decider, types::CandidateId};

pub struct PluralityDecider;

impl Decider for PluralityDecider {
    fn decide(scores: &[usize]) -> Vec<CandidateId> {
        let mut cur_max = 0;
        let mut winners = vec![];

        for (idx, &score) in scores.iter().enumerate() {
            if score > cur_max {
                cur_max = score;
                winners = vec![idx];
            } else if score == cur_max {
                winners.push(idx);
            }
        }

        winners
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_one_winner() {
        let scores = [0, 1, 0, 2];

        assert_eq!(vec![3], PluralityDecider::decide(&scores));
    }

    #[test]
    fn test_several_winners() {
        let scores = [0, 1, 0, 1];

        assert_eq!(vec![1, 3], PluralityDecider::decide(&scores));
    }

    #[test]
    fn test_all_winners() {
        let scores = [1, 1, 1, 1, 1];

        assert_eq!(vec![0, 1, 2, 3, 4], PluralityDecider::decide(&scores));
    }
}
