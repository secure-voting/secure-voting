mod common;

use voting_core::{
    prelude::{AntiPluralityRule, RuleOutcome, VotingRuleExec},
    profile::CandidateId,
    tie_breaker::fallthrough::FallthroughTieBreaker,
};

use crate::common::{CHATTANOOGA, NASHVILLE, construct_tennessee_wiki_example};

#[test]
fn test_wiki_example() {
    let profile = construct_tennessee_wiki_example();
    let scorer = AntiPluralityRule::<FallthroughTieBreaker>::default();

    assert_eq!(
        RuleOutcome::MultipleWinners(vec![
            CandidateId::new(NASHVILLE),
            CandidateId::new(CHATTANOOGA)
        ]),
        scorer.execute(&profile).unwrap()
    );
}
