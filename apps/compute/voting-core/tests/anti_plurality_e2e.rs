mod common;

use voting_core::{
    models::BallotData,
    models::candidate_id::CandidateId,
    prelude::{AntiPluralityRule, RuleOutcome, VotingRuleExec},
};

use crate::common::{CHATTANOOGA, NASHVILLE, construct_tennessee_wiki_example};

#[test]
fn wiki_tennessee_example() {
    let profile = construct_tennessee_wiki_example();
    let scorer = AntiPluralityRule::default();

    assert_eq!(
        RuleOutcome::MultipleWinners(vec![
            CandidateId::new(NASHVILLE, "NASHVILLE"),
            CandidateId::new(CHATTANOOGA, "CHATTANOOGA")
        ]),
        scorer
            .execute(&profile)
            .expect("Scorer failed, but shouldn't have.")
            .0
    );
}

#[test]
fn simple_antiplurality() {
    let ballots: Vec<BallotData> = vec![
        BallotData::Simple(vec![CandidateId::new(0, "A"), CandidateId::new(2, "C"), CandidateId::new(1, "B")]),
        BallotData::Simple(vec![CandidateId::new(0, "A"), CandidateId::new(1, "B"), CandidateId::new(2, "C")]),
        BallotData::Simple(vec![CandidateId::new(2, "C"), CandidateId::new(0, "A"), CandidateId::new(1, "B")]),
    ];
    let profile = (
        ballots,
        vec!["A".into(), "B".into(), "C".into()],
    )
        .try_into()
        .expect("Profile is constructed incorrectly, revise test example");
    let scorer = AntiPluralityRule::default();

    assert_eq!(
        RuleOutcome::UniqueWinner(CandidateId::new(0, "A")),
        scorer
            .execute(&profile)
            .expect("Scorer failed, but shouldn't have.")
            .0
    );
}

#[test]
fn multiple_winners() {
    let ballots: Vec<BallotData> = vec![
        BallotData::Simple(vec![CandidateId::new(0, "C0"), CandidateId::new(2, "C2"), CandidateId::new(1, "C1")]),
        BallotData::Simple(vec![CandidateId::new(0, "C0"), CandidateId::new(1, "C1"), CandidateId::new(2, "C2")]),
        BallotData::Simple(vec![CandidateId::new(2, "C2"), CandidateId::new(1, "C1"), CandidateId::new(0, "C0")]),
    ];
    let profile = (
        ballots,
        vec!["C0".into(), "C1".into(), "C2".into()],
    )
        .try_into()
        .expect("Profile is constructed incorrectly, revise test example");
    let scorer = AntiPluralityRule::default();

    assert_eq!(
        RuleOutcome::MultipleWinners(
            (0..3)
                .map(|i| CandidateId::new(i, format!("C{i}")))
                .collect()
        ),
        scorer
            .execute(&profile)
            .expect("Scorer failed, but shouldn't have.")
            .0
    );
}
