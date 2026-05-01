mod common;

use common::*;
use voting_core::{
    models::{BallotData, scoring::ScoreBallot},
    prelude::*,
    voting_rules::scoring::ScoreRule,
};

#[allow(clippy::expect_used)]
fn construct_tennessee_wiki_example() -> Profile<ScoreBallot> {
    let memphis = CandidateId::new(MEMPHIS, "MEMPHIS");
    let nashville = CandidateId::new(NASHVILLE, "NASHVILLE");
    let chattanooga = CandidateId::new(CHATTANOOGA, "CHATTANOOGA");
    let knoxville = CandidateId::new(KNOXVILLE, "KNOXVILLE");

    let mut votes = Vec::<BallotData>::with_capacity(100);

    (0..42).for_each(|_| {
        votes.push(BallotData::Scoring(vec![
            (memphis.clone(), 10),
            (nashville.clone(), 4),
            (chattanooga.clone(), 2),
            (knoxville.clone(), 0),
        ]));
    });
    (0..26).for_each(|_| {
        votes.push(BallotData::Scoring(vec![
            (memphis.clone(), 0),
            (nashville.clone(), 10),
            (chattanooga.clone(), 4),
            (knoxville.clone(), 2),
        ]));
    });
    (0..15).for_each(|_| {
        votes.push(BallotData::Scoring(vec![
            (memphis.clone(), 0),
            (nashville.clone(), 6),
            (chattanooga.clone(), 10),
            (knoxville.clone(), 6),
        ]));
    });
    (0..17).for_each(|_| {
        votes.push(BallotData::Scoring(vec![
            (memphis.clone(), 0),
            (nashville.clone(), 5),
            (chattanooga.clone(), 7),
            (knoxville.clone(), 10),
        ]));
    });

    let names = vec![
        "MEMPHIS".to_owned(),
        "NASHVILLE".to_owned(),
        "CHATTANOOGA".to_owned(),
        "KNOXVILLE".to_owned(),
    ];

    Profile::try_from((votes, names))
        .expect("Profile is constructed incorrectly, revise test example")
}

#[test]
fn wiki_tennesee_example() {
    let profile = construct_tennessee_wiki_example();
    let scorer = ScoreRule::default();

    assert_eq!(
        RuleOutcome::UniqueWinner(CandidateId::new(NASHVILLE, "NASHVILLE")),
        scorer
            .execute(&profile)
            .expect("Scorer failed, but shouldn't have.")
            .0
    );
}
