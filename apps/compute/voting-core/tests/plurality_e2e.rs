mod common;

use voting_core::{
    prelude::*, profile::CandidateId, tie_breaker::fallthrough::FallthroughTieBreaker,
};

use crate::common::{MEMPHIS, construct_tennessee_wiki_example};

#[test]
fn test_wiki_example() {
    let profile = construct_tennessee_wiki_example();
    let scorer = PluralityRule::default();

    assert_eq!(
        RuleOutcome::UniqueWinner(CandidateId::new(MEMPHIS)),
        scorer.execute(&profile).unwrap()
    );
}

#[test]
fn test_simple_plurality() {
    let profile = vec![vec![0, 1, 2], vec![0, 2, 1]].try_into().unwrap();
    let scorer = PluralityRule::default();

    assert_eq!(
        RuleOutcome::UniqueWinner(CandidateId::new(0)),
        scorer.execute(&profile).unwrap()
    );
}

#[test]
fn test_multiple_winners() {
    let profile = vec![vec![0, 1, 2], vec![0, 2, 1], vec![1, 0, 2], vec![1, 2, 0]]
        .try_into()
        .unwrap();
    let scorer = PluralityRule::default();

    assert_eq!(
        RuleOutcome::MultipleWinners(vec![CandidateId::new(0), CandidateId::new(1)]),
        scorer.execute(&profile).unwrap()
    );
}
