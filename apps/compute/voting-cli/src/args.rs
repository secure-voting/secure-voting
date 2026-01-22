use std::str::FromStr;

use anyhow::anyhow;
use clap::Parser;

#[derive(Parser, Debug)]
#[command(version, about)]
pub struct Args {
    /// Input file path
    #[arg(short, long)]
    file: String,
    /// Input file format
    #[arg(short = 't', long = "type")]
    format: String,
    /// Voting rule name
    #[arg(short, long)]
    rule: RuleName,
}

#[derive(Debug, Clone, Copy)]
pub enum RuleName {
    /// Plurality rule.
    ///
    /// Candidates are chosen by the total number of first-place votes for them.
    Plurality,
    /// Approval rule.
    ///
    /// Candidates are chosen by the total number of votes in the first q places for each ballot.
    Approval(usize),
    /// Inverse plurality rule.
    ///
    /// Candidates are chosen by the least number of last-place votes for them.
    InversePlurality,
    /// Borda rule.
    ///
    /// Candidates are scored per ballot, getting n points per first place and 0 for last.
    /// Winners are chosen by most total score.
    Borda,
}

impl FromStr for RuleName {
    type Err = anyhow::Error;

    fn from_str(s: &str) -> Result<Self, Self::Err> {
        let binding = s.to_lowercase();
        let s = binding.as_str();

        if s.starts_with("approval") {
            let postfix = s.strip_prefix("approval").unwrap();
            let x = postfix.parse().map_err(|_| {
                anyhow!(
                    "Can't parse Q for approval, expected number, got: {}",
                    postfix
                )
            })?;
            return Ok(Self::Approval(x));
        }

        match s {
            "plurality" => Ok(Self::Plurality),
            "inverseplurality" => Ok(Self::InversePlurality),
            "borda" => Ok(Self::Borda),
            name => Err(anyhow!("Unknown rule name: {}", name)),
        }
    }
}
