mod common;

use voting_core::prelude::*;

use crate::common::{NASHVILLE, construct_tennessee_wiki_example};

#[test]
fn test_wiki_example_copeland_i() {
    let profile = construct_tennessee_wiki_example();
    let scorer = CopelandIRule::default();

    assert_eq!(
        RuleOutcome::UniqueWinner(CandidateId::new(NASHVILLE)),
        scorer.execute(&profile).unwrap()
    );
}

#[test]
fn test_wiki_example_copeland_ii() {
    let profile = construct_tennessee_wiki_example();
    let scorer = CopelandIIRule::default();

    assert_eq!(
        RuleOutcome::UniqueWinner(CandidateId::new(NASHVILLE)),
        scorer.execute(&profile).unwrap()
    );
}

#[test]
fn test_wiki_example_copeland_iii() {
    let profile = construct_tennessee_wiki_example();
    let scorer = CopelandIIIRule::default();

    assert_eq!(
        RuleOutcome::UniqueWinner(CandidateId::new(NASHVILLE)),
        scorer.execute(&profile).unwrap()
    );
}
