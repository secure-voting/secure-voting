//! This module aids in holding elections when candidate have names.
//!
//! All the other parts of the library operate on anonymised data, using candidate IDs.
//! This creates a bridge between real-world usage and this library.

use std::error::Error;

use crate::{
    models::BallotData,
    prelude::{Profile, VotingRuleExec},
    voting_rules::{Metrics, Protocol},
};

/// Execute an election with named candidates and a chosen rule.
///
/// # Errors
///
/// An error can occur if the supplied input data doesn't represent a correct voting profile.
pub fn run_election<T, VRE: VotingRuleExec<T>>(
    ballots: Vec<BallotData>,
    names: Vec<String>,
    rule: &VRE,
) -> anyhow::Result<(Vec<String>, Metrics, Protocol)>
where
    VRE::Error: Error + Send + Sync + 'static,
    Profile<T>: TryFrom<(Vec<BallotData>, Vec<String>)>,
    <Profile<T> as TryFrom<(Vec<BallotData>, Vec<String>)>>::Error: Error + Send + Sync + 'static,
{
    let profile = Profile::try_from((ballots, names))?;

    let result = rule.execute(&profile)?;

    let outcome = result
        .0
        .candidates()
        .iter()
        .map(|x| x.get_name().to_owned())
        .collect();

    Ok((outcome, result.1, result.2))
}
