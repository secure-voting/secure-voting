use voting_core::prelude::*;

use crate::common::{CHATTANOOGA, construct_tennessee_wiki_example};

mod common;

#[test]
fn test_wiki_example() {
    let profile = construct_tennessee_wiki_example();
    let scorer = ApprovalRule::<2>::default();

    assert_eq!(
        RuleOutcome::UniqueWinner(CandidateId::new(CHATTANOOGA)),
        scorer.execute(&profile).unwrap()
    );
}
