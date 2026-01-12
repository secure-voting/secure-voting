use voting_core::{
    prelude::*, scorer::approval::ApprovalScorerError, voting_rules::voting_rule::VotingRuleError,
};

use crate::common::{CHATTANOOGA, MEMPHIS, NASHVILLE, construct_tennessee_wiki_example};

mod common;

#[test]
fn wiki_example_q_1() {
    let profile = construct_tennessee_wiki_example();
    let scorer = ApprovalRule::<1>::default();

    assert_eq!(
        RuleOutcome::UniqueWinner(CandidateId::new(MEMPHIS)),
        scorer
            .execute(&profile)
            .expect("Scorer failed, but shouldn't have.")
    );
}

#[test]
fn wiki_example_q_2() {
    let profile = construct_tennessee_wiki_example();
    let scorer = ApprovalRule::<2>::default();

    assert_eq!(
        RuleOutcome::UniqueWinner(CandidateId::new(NASHVILLE)),
        scorer
            .execute(&profile)
            .expect("Scorer failed, but shouldn't have.")
    );
}

#[test]
fn wiki_example_q_3() {
    let profile = construct_tennessee_wiki_example();
    let scorer = ApprovalRule::<3>::default();

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
fn error_q_too_big() {
    let profile = construct_tennessee_wiki_example();
    let scorer = ApprovalRule::<6>::default();

    assert!(matches!(
        scorer.execute(&profile),
        Err(VotingRuleError::ScoringError(ApprovalScorerError))
    ));
}
