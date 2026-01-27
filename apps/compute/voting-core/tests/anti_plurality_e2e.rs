mod common;

use voting_core::{
    prelude::{AntiPluralityRule, RuleOutcome, VotingRuleExec},
    profile::CandidateId,
};

use crate::common::{CHATTANOOGA, NASHVILLE, construct_tennessee_wiki_example};

#[test]
fn wiki_tennessee_example() {
    let profile = construct_tennessee_wiki_example();
    let scorer = AntiPluralityRule::default();

    assert_eq!(
        RuleOutcome::MultipleWinners(vec![
            CandidateId::new(NASHVILLE),
            CandidateId::new(CHATTANOOGA)
        ]),
        scorer
            .execute(&profile)
            .expect("Scorer failed, but shouldn't have.")
    );
}

#[test]
fn simple_antiplurality() {
    let profile = vec![vec![0, 2, 1], vec![0, 1, 2], vec![2, 0, 1]]
        .try_into()
        .expect("Profile is constructed incorrectly, revise test example");
    let scorer = AntiPluralityRule::default();

    assert_eq!(
        RuleOutcome::UniqueWinner(CandidateId::new(0)),
        scorer
            .execute(&profile)
            .expect("Scorer failed, but shouldn't have.")
    );
}

#[test]
fn multiple_winners() {
    let profile = vec![vec![0, 2, 1], vec![0, 1, 2], vec![2, 1, 0]]
        .try_into()
        .expect("Profile is constructed incorrectly, revise test example");
    let scorer = AntiPluralityRule::default();

    assert_eq!(
        RuleOutcome::MultipleWinners((0..3).map(CandidateId::new).collect()),
        scorer
            .execute(&profile)
            .expect("Scorer failed, but shouldn't have.")
    );
}
