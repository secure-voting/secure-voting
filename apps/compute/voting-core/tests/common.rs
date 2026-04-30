use std::sync::Once;

use ctor::ctor;
use tracing_subscriber::{EnvFilter, fmt};
use voting_core::models::{
    BallotData, candidate_id::CandidateId, profile::Profile, ranking::RankingBallot,
};

pub const MEMPHIS: usize = 0;
pub const NASHVILLE: usize = 1;
pub const CHATTANOOGA: usize = 2;
pub const KNOXVILLE: usize = 3;

#[must_use]
/// Construct a tennesse wiki example (used in a lot of the voting rules' pages).
///
/// # Panics
///
/// Shouldn't panic if the profile implementation is unchanged.
#[expect(
    clippy::expect_used,
    reason = "This is a utility function solely used by tests. Expect here is justified."
)]
pub fn construct_tennessee_wiki_example() -> Profile<RankingBallot> {
    let memphis = CandidateId::new(MEMPHIS, "MEMPHIS");
    let nashville = CandidateId::new(NASHVILLE, "NASHVILLE");
    let chattanooga = CandidateId::new(CHATTANOOGA, "CHATTANOOGA");
    let knoxville = CandidateId::new(KNOXVILLE, "KNOXVILLE");

    let mut votes = Vec::<BallotData>::with_capacity(100);

    (0..42).for_each(|_| {
        votes.push(BallotData::Simple(vec![
            memphis.clone(),
            nashville.clone(),
            chattanooga.clone(),
            knoxville.clone(),
        ]))
    });
    (0..26).for_each(|_| {
        votes.push(BallotData::Simple(vec![
            nashville.clone(),
            chattanooga.clone(),
            knoxville.clone(),
            memphis.clone(),
        ]))
    });
    (0..15).for_each(|_| {
        votes.push(BallotData::Simple(vec![
            chattanooga.clone(),
            knoxville.clone(),
            nashville.clone(),
            memphis.clone(),
        ]))
    });
    (0..17).for_each(|_| {
        votes.push(BallotData::Simple(vec![
            knoxville.clone(),
            chattanooga.clone(),
            nashville.clone(),
            memphis.clone(),
        ]))
    });

    Profile::try_from((
        votes,
        vec![
            "MEMPHIS".into(),
            "NASHVILLE".into(),
            "CHATTANOOGA".into(),
            "KNOXVILLE".into(),
        ],
    ))
    .expect("Profile is constructed incorrectly, revise test example.")
}

static INIT: Once = Once::new();

#[ctor]
pub fn init_tracing() {
    INIT.call_once(|| {
        let subscriber = fmt()
            .with_env_filter(EnvFilter::from_default_env())
            .with_test_writer()
            .with_line_number(true)
            .with_file(true)
            .finish();
        let _ = tracing::subscriber::set_global_default(subscriber);
    });
}
