use anyhow::anyhow;
use clap::Parser;
use voting_core::prelude::*;

use crate::args::{Args, RuleName};

mod args;
mod models;

fn main() {
    let args = Args::parse();
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
    }
}
