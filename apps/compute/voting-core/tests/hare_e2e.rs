mod common;

use voting_core::prelude::*;

use crate::common::{KNOXVILLE, construct_tennessee_wiki_example};

#[test]
fn test_wiki_example() {
    tracing::info!("test");
    let profile = construct_tennessee_wiki_example();
    let scorer = HareRule::default();

    assert_eq!(
        RuleOutcome::UniqueWinner(CandidateId::new(KNOXVILLE)),
        scorer.execute(&profile).unwrap()
    );
}
