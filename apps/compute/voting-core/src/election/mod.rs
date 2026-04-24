//! This module aids in holding elections when candidate have names.
//!
//! All the other parts of the library operate on anonymised data, using candidate IDs.
//! This creates a bridge between real-world usage and this library.

use std::error::Error;

use crate::{
    prelude::{Profile, VotingRuleExec},
    voting_rules::{Metrics, Protocol},
};

/// Execute an election with named candidates and a chosen rule.
///
/// # Errors
///
/// An error can occur if the supplied input data doesn't represent a correct voting profile.
#[allow(
    clippy::missing_panics_doc,
    reason = "The panic cannot occur, the element that was in the array, could be found by position()"
)]
pub fn run_election<T, VRE: VotingRuleExec<T>>(
    ballots: Vec<Vec<String>>,
    rule: &VRE,
) -> anyhow::Result<(Vec<String>, Metrics, Protocol)>
where
    VRE::Error: Error + Send + Sync + 'static,
    Profile<T>: TryFrom<(Vec<Vec<usize>>, Vec<String>)>,
    <Profile<T> as TryFrom<(Vec<Vec<usize>>, Vec<String>)>>::Error: Error + Send + Sync + 'static,
{
    let mut name_set: Vec<String> = Vec::new();
    for ballot in &ballots {
        for vote in ballot {
            if !name_set.contains(vote) {
                name_set.push(vote.clone());
            }
        }
    }

    #[allow(clippy::unwrap_used)]
    let vote_data: Vec<Vec<usize>> = ballots
        .iter()
        .map(|ballot| {
            ballot
                .iter()
                .map(|vote| name_set.iter().position(|n| n == vote).unwrap())
                .collect()
        })
        .collect();

    let profile = Profile::try_from((vote_data, name_set))?;

    let result = rule.execute(&profile)?;

    let outcome = result
        .0
        .candidates()
        .iter()
        .map(|x| x.get_name().to_owned())
        .collect();

    Ok((outcome, result.1, result.2))
}
