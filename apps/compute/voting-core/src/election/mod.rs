//! This module aids in holding elections when candidate have names.
//!
//! All the other parts of the library operate on anonymised data, using candidate IDs.
//! This creates a bridge between real-world usage and this library.

use std::{collections::HashMap, error::Error};

use crate::prelude::{Profile, VotingRuleExec};

/// Execute an election with named candidates and a chosen rule.
///
/// # Errors
///
/// An error can occur if the supplied input data doesn't represent a correct voting profile.
pub fn run_election<T, VRE: VotingRuleExec<T>>(
    input_data: Vec<Vec<impl Into<String>>>,
    rule: &VRE,
) -> anyhow::Result<Vec<String>>
where
    VRE::Error: Error + Send + Sync + 'static,
    Profile<T>: TryFrom<Vec<Vec<usize>>>,
    <Profile<T> as TryFrom<Vec<Vec<usize>>>>::Error: Error + Send + Sync + 'static,
{
    let mut cand_to_id: HashMap<String, usize> = HashMap::new();
    let mut id_to_cand = vec![];
    let mut vote_data = Vec::with_capacity(input_data.len());

    let mut cand_id = 0;
    for (idx, ballot) in input_data.into_iter().enumerate() {
        vote_data.push(Vec::with_capacity(ballot.len()));
        for vote in ballot {
            let vote = vote.into();

            if let Some(&cur_id) = cand_to_id.get(&vote) {
                vote_data[idx].push(cur_id);
                continue;
            }

            cand_to_id.insert(vote.clone(), cand_id);
            id_to_cand.push(vote);
            vote_data[idx].push(cand_id);
            cand_id += 1;
        }
    }

    let profile = Profile::try_from(vote_data)?;

    let result = rule.execute(&profile)?;

    Ok(result
        .candidates()
        .iter()
        .map(|x| id_to_cand[x.into_inner()].clone())
        .collect())
}
