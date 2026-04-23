mod common;

use voting_core::prelude::*;

use crate::common::{NASHVILLE, construct_tennessee_wiki_example};

#[test]
fn wiki_example_condorcet_winner() {
    // let profile = construct_tennessee_wiki_example();
    // let scorer = BlackRule::default();

    // assert_eq!(
    //     RuleOutcome::UniqueWinner(CandidateId::new(NASHVILLE, "NASHVILLE")),
    //     scorer
    //         .execute(&profile)
    //         .expect("Scorer failed, but shouldn't have.")
    //         .0
    // );
}
