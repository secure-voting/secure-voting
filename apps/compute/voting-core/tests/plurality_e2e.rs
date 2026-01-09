mod common;

use voting_core::{
    prelude::{PluralityRule, RuleOutcome, VotingRuleExec},
    profile::CandidateId,
    tie_breaker::fallthrough::FallthroughTieBreaker,
};

use crate::common::{MEMPHIS, construct_tennessee_wiki_example};

#[test]
fn test_wiki_example() {
    let profile = construct_tennessee_wiki_example();
    let scorer = PluralityRule::<FallthroughTieBreaker>::default();

    assert_eq!(
        RuleOutcome::UniqueWinner(CandidateId::new(MEMPHIS)),
        scorer.execute(&profile).unwrap()
    );
}
