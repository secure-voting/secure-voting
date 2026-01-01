use crate::scorer::Scorer;

pub struct PluralityScorer;

impl Scorer for PluralityScorer {
    fn score_ballot(ballot: &[usize], scores: &mut [usize]) {
        if ballot.is_empty() {
            return;
        }
        scores[ballot[0]] += 1;
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use test_case::test_case;

    #[test_case(vec![vec![]], vec![]; "empty votes")]
    #[test_case(vec![vec![1, 0], vec![0, 1], vec![1, 0]], vec![1, 2]; "simple plurality")]
    fn test_simple_plurality(votes: Vec<Vec<usize>>, answer: Vec<usize>) {
        assert_eq!(
            answer,
            PluralityScorer::compute_score(&votes.try_into().unwrap())
        );
    }
}
