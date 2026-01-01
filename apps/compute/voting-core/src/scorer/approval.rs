use crate::scorer::Scorer;

pub struct ApprovalScorer<const Q: usize>;

impl<const Q: usize> Scorer for ApprovalScorer<Q> {
    fn score_ballot(ballot: &[usize], scores: &mut [usize]) {
        if ballot.len() < Q {
            return;
        }
        (0..Q).for_each(|i| scores[ballot[i]] += 1);
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use test_case::test_case;

    #[test_case(vec![vec![]], vec![]; "empty votes")]
    #[test_case(vec![vec![1, 0], vec![0, 1], vec![1, 0]], vec![3, 3]; "count all")]
    #[test_case(vec![vec![1, 0, 2], vec![0, 2, 1], vec![0, 2, 1]], vec![3, 1, 2]; "count top 2")]
    fn test_simple_plurality(votes: Vec<Vec<usize>>, answer: Vec<usize>) {
        assert_eq!(
            answer,
            ApprovalScorer::<2>::compute_score(&votes.try_into().unwrap())
        );
    }
}
