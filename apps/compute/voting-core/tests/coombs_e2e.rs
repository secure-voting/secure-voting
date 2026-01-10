mod common;

use voting_core::{
    prelude::{CoombsRule, RuleOutcome, VotingRuleExec},
    profile::CandidateId,
};

use crate::common::{NASHVILLE, construct_tennessee_wiki_example};

#[test]
fn test_wiki_example() {
    tracing::info!("test");
    let profile = construct_tennessee_wiki_example();
    let scorer = CoombsRule::default();

    assert_eq!(
        RuleOutcome::UniqueWinner(CandidateId::new(NASHVILLE)),
        scorer.execute(&profile).unwrap()
    );
}
