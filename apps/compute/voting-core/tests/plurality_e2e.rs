mod common;

use voting_core::{models::candidate_id::CandidateId, prelude::*};

use crate::common::{MEMPHIS, construct_tennessee_wiki_example};

#[test]
fn wiki_tennessee_example() {
    let profile = construct_tennessee_wiki_example();
    let scorer = PluralityRule::default();

    assert_eq!(
        RuleOutcome::UniqueWinner(CandidateId::new(MEMPHIS)),
        scorer
            .execute(&profile)
            .expect("Scorer failed, but shouldn't have.")
    );
}

#[test]
fn simple_plurality() {
    let profile = vec![vec![0, 1, 2], vec![0, 2, 1]]
        .try_into()
        .expect("Profile is constructed incorrectly, revise test example");
    let scorer = PluralityRule::default();

    assert_eq!(
        RuleOutcome::UniqueWinner(CandidateId::new(0)),
        scorer
            .execute(&profile)
            .expect("Scorer failed, but shouldn't have.")
    );
}

#[test]
fn multiple_winners() {
    let profile = vec![vec![0, 1, 2], vec![0, 2, 1], vec![1, 0, 2], vec![1, 2, 0]]
        .try_into()
        .expect("Profile is constructed incorrectly, revise test example");
    let scorer = PluralityRule::default();

    assert_eq!(
        RuleOutcome::MultipleWinners(vec![CandidateId::new(0), CandidateId::new(1)]),
        scorer
            .execute(&profile)
            .expect("Scorer failed, but shouldn't have.")
    );
}
