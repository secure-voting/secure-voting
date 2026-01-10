mod common;

use voting_core::prelude::*;

use crate::common::{NASHVILLE, construct_tennessee_wiki_example};

#[test]
fn test_wiki_example() {
    let profile = construct_tennessee_wiki_example();
    let scorer = MinmaxRule::default();

    assert_eq!(
        RuleOutcome::UniqueWinner(CandidateId::new(NASHVILLE)),
        scorer.execute(&profile).unwrap()
    );
}
