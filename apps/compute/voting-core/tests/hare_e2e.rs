mod common;

use voting_core::prelude::*;

use crate::common::{KNOXVILLE, construct_tennessee_wiki_example};

#[test]
fn wiki_tennessee_example() {
    tracing::info!("test");
    let profile = construct_tennessee_wiki_example();
    let scorer = HareRule::default();

    assert_eq!(
        RuleOutcome::UniqueWinner(CandidateId::new(KNOXVILLE)),
        scorer
            .execute(&profile)
            .expect("Scorer failed, but shouldn't have.")
    );
}
