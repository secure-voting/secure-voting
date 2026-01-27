mod common;

use voting_core::prelude::*;

use crate::common::{NASHVILLE, construct_tennessee_wiki_example};

#[test]
fn wiki_tennessee_example_copeland_i() {
    let profile = construct_tennessee_wiki_example();
    let scorer = CopelandIRule::default();

    assert_eq!(
        RuleOutcome::UniqueWinner(CandidateId::new(NASHVILLE)),
        scorer
            .execute(&profile)
            .expect("Scorer failed, but shouldn't have")
    );
}

#[test]
fn wiki_tennessee_example_copeland_ii() {
    let profile = construct_tennessee_wiki_example();
    let scorer = CopelandIIRule::default();

    assert_eq!(
        RuleOutcome::UniqueWinner(CandidateId::new(NASHVILLE)),
        scorer
            .execute(&profile)
            .expect("Scorer failed, but shouldn't have")
    );
}

#[test]
fn wiki_tennessee_example_copeland_iii() {
    let profile = construct_tennessee_wiki_example();
    let scorer = CopelandIIIRule::default();

    assert_eq!(
        RuleOutcome::UniqueWinner(CandidateId::new(NASHVILLE)),
        scorer
            .execute(&profile)
            .expect("Scorer failed, but shouldn't have")
    );
}
