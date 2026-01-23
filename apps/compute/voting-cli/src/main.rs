use std::collections::HashMap;
use std::io;

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
    }
}

fn compute_result(rule_enum: &RuleName, input_data: &Profile) -> anyhow::Result<RuleOutcome> {
    use RuleName::*;
    match rule_enum {
        Plurality => Ok(PluralityRule::default().execute(input_data)?),
        Approval2 => Ok(ApprovalRule::<2>::default().execute(input_data)?),
        Approval3 => Ok(ApprovalRule::<3>::default().execute(input_data)?),
        InversePlurality => Ok(AntiPluralityRule::default().execute(input_data)?),
        Borda => Ok(BordaRule::default().execute(input_data)?),
        Black => Ok(BlackRule::default().execute(input_data)?),
        CopelandI => Ok(CopelandIRule::default().execute(input_data)?),
        CopelandII => Ok(CopelandIIRule::default().execute(input_data)?),
        CopelandIII => Ok(CopelandIIIRule::default().execute(input_data)?),
    }
}
