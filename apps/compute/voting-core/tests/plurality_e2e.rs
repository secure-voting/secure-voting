mod common;

use voting_core::{models::candidate_id::CandidateId, prelude::*};

use crate::common::{MEMPHIS, construct_tennessee_wiki_example};

#[test]
fn wiki_tennessee_example() {
    let profile = construct_tennessee_wiki_example();
    let scorer = PluralityRule::default();

    assert_eq!(
        RuleOutcome::UniqueWinner(CandidateId::new(MEMPHIS, "MEMPHIS")),
        scorer
            .execute(&profile)
            .expect("Scorer failed, but shouldn't have.")
            .0
    );
}

#[test]
fn simple_plurality() {
    let profile = (
        vec![vec![0, 1, 2], vec![0, 2, 1]],
        vec!["A".into(), "B".into(), "C".into()],
    )
        .try_into()
        .expect("Profile is constructed incorrectly, revise test example");
    let scorer = PluralityRule::default();

    assert_eq!(
        RuleOutcome::UniqueWinner(CandidateId::new(0, "A")),
        scorer
            .execute(&profile)
            .expect("Scorer failed, but shouldn't have.")
            .0
    );
}

#[test]
fn multiple_winners() {
    let profile = (
        vec![vec![0, 1, 2], vec![0, 2, 1], vec![1, 0, 2], vec![1, 2, 0]],
        vec!["A".into(), "B".into(), "C".into()],
    )
        .try_into()
        .expect("Profile is constructed incorrectly, revise test example");
    let scorer = PluralityRule::default();

    assert_eq!(
        RuleOutcome::MultipleWinners(vec![CandidateId::new(0, "A"), CandidateId::new(1, "B")]),
        scorer
            .execute(&profile)
            .expect("Scorer failed, but shouldn't have.")
            .0
    );
}
