use std::collections::HashMap;
use std::io;

use clap::Parser;
use voting_core::models::ranking::RankingBallot;
use voting_core::prelude::*;

use crate::args::{Args, InputFormat, RuleName};
use crate::models::{ProfileParser, cvr::CVRParser};

mod args;
mod models;

fn main() -> anyhow::Result<()> {
    let args = Args::parse();
    let file_contents = std::fs::read_to_string(args.file)?;
    let (profile, mapping) = get_profile_and_mappings(args.format, file_contents.as_bytes())?;
    let result = compute_result(args.rule, &profile)?;

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
) -> anyhow::Result<(Profile<RankingBallot>, HashMap<CandidateId, String>)> {
    match format {
        InputFormat::Cvr => Ok(CVRParser.parse(reader)?),
    }
}

fn compute_result(
    rule_enum: RuleName,
    input_data: &Profile<RankingBallot>,
) -> anyhow::Result<RuleOutcome> {
    match rule_enum {
        RuleName::Plurality => Ok(PluralityRule::default().execute(input_data)?.0),
        RuleName::Approval2 => Ok(ApprovalRule::<2>::default().execute(input_data)?.0),
        RuleName::Approval3 => Ok(ApprovalRule::<3>::default().execute(input_data)?.0),
        RuleName::InversePlurality => Ok(AntiPluralityRule::default().execute(input_data)?.0),
        RuleName::Borda => Ok(BordaRule::default().execute(input_data)?.0),
        // RuleName::Black => Ok(BlackRule::default().execute(input_data)?.0),
        RuleName::CopelandI => Ok(CopelandIRule::default().execute(input_data)?.0),
        RuleName::CopelandII => Ok(CopelandIIRule::default().execute(input_data)?.0),
        RuleName::CopelandIII => Ok(CopelandIIIRule::default().execute(input_data)?.0),
        RuleName::Simpson => Ok(SimpsonRule::default().execute(input_data)?.0),
        RuleName::Minmax => Ok(MinmaxRule::default().execute(input_data)?.0),
        RuleName::Hare => Ok(HareRule::default().execute(input_data)?.0),
        RuleName::Nanson => Ok(NansonRule::default().execute(input_data)?.0),
        // RuleName::Coombs => Ok(CoombsRule::default().execute(input_data)?.0),
        RuleName::InverseBorda => Ok(InverseBordaRule::default().execute(input_data)?.0),
        _ => Err(anyhow::format_err!("Unimplemented")),
    }
}
