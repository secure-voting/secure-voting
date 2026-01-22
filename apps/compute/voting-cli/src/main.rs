use std::collections::HashMap;
use std::io;

use anyhow::anyhow;
use clap::Parser;
use voting_core::prelude::*;

use crate::args::{Args, InputFormat, RuleName};
use crate::models::{ProfileParser, cvr::CVRParser};

mod args;
mod models;

fn main() -> anyhow::Result<()> {
    let args = Args::parse();
    let file_contents = std::fs::read_to_string(args.file)?;
    let (profile, mapping) = get_profile_and_mappings(args.format, file_contents.as_bytes())?;
    let result = compute_result(&args.rule, &profile)?;

    match result {
        RuleOutcome::UniqueWinner(candidate_id) => {
            println!("A unique winner is determined: {}", mapping[&candidate_id]);
        }
        RuleOutcome::MultipleWinners(candidate_ids) => println!(
            "A unique winner couldn't be found, but here are candidates tied for a win: {:?}",
            candidate_ids
                .iter()
                .map(|x| &mapping[x])
                .cloned()
                .collect::<Vec<_>>()
        ),
    }

    Ok(())
}

fn get_profile_and_mappings<R: io::Read>(
    format: InputFormat,
    reader: R,
) -> anyhow::Result<(Profile, HashMap<CandidateId, String>)> {
    match format {
        InputFormat::Cvr => Ok(CVRParser.parse(reader)?),
        _ => Err(anyhow!("Unsupported input format")),
    }
}

fn compute_result(rule_enum: &RuleName, input_data: &Profile) -> anyhow::Result<RuleOutcome> {
    match rule_enum {
        RuleName::Plurality => Ok(PluralityRule::default().execute(input_data)?),
        RuleName::Approval(q) => match q {
            2 => Ok(ApprovalRule::<2>::default().execute(input_data)?),
            3 => Ok(ApprovalRule::<3>::default().execute(input_data)?),
            _ => Err(anyhow!(
                "Approval rule doesn't support q different from 2 or 3."
            )),
        },
        RuleName::InversePlurality => Ok(AntiPluralityRule::default().execute(input_data)?),
        RuleName::Borda => Ok(BordaRule::default().execute(input_data)?),
        _ => Err(anyhow!("Unsupported rule")),
    }
}
