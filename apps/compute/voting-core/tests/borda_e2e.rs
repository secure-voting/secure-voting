use voting_core::prelude::*;

use crate::common::{NASHVILLE, construct_tennessee_wiki_example};

mod common;

#[test]
fn wiki_tennesee_example() {
    let profile = construct_tennessee_wiki_example();
    let scorer = BordaRule::default();

    assert_eq!(
        RuleOutcome::UniqueWinner(CandidateId::new(NASHVILLE, "NASHVILLE")),
        scorer
            .execute(&profile)
            .expect("Scorer failed, but shouldn't have.")
            .0
    );
}
