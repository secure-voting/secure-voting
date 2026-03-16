mod common;

use voting_core::{
    models::candidate_id::CandidateId,
    prelude::{CoombsRule, RuleOutcome, VotingRuleExec},
};

use crate::common::{NASHVILLE, construct_tennessee_wiki_example};

#[test]
fn wiki_tennesee_example() {
    tracing::info!("test");
    let profile = construct_tennessee_wiki_example();
    let scorer = CoombsRule::default();

    assert_eq!(
        RuleOutcome::UniqueWinner(CandidateId::new(NASHVILLE)),
        scorer
            .execute(&profile)
            .expect("Scorer failed, but shouldn't have.")
    );
}
