use std::{path::PathBuf, str::FromStr};

use anyhow::anyhow;
use clap::{Parser, ValueEnum};

#[derive(Parser, Debug)]
#[command(version, about)]
pub struct Args {
    /// Input file path
    #[arg(short, long)]
    pub file: PathBuf,
    /// Input file format
    #[arg(short = 't', long = "type", value_enum)]
    pub format: InputFormat,
    /// Voting rule name
    #[arg(short, long)]
    pub rule: RuleName,
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
    /// Borda's rule.
    ///
    /// Candidates are scored per ballot, getting n points per first place and 0 for last.
    /// Winners are chosen by most total score.
    Borda,
    /// Black's rule.
    ///
    /// If there is a Condorcet winner, choose them, otherwise the Borda's rule is used.
    Black,
    /// Copeland I rule.
    ///
    /// Candidates are scored by the number of strict head-to-head wins.
    CopelandI,
    /// Copeland II rule.
    ///
    /// Candidates are scored by the difference between the number of strict wins and strict losses.
    CopelandII,
    /// Copeland III rule.
    ///
    /// Candidates are chosen by the margin of winning in each head-to-head.
    CopelandIII,
}

impl FromStr for RuleName {
    type Err = anyhow::Error;

    fn from_str(s: &str) -> Result<Self, Self::Err> {
        let binding = s.to_lowercase();
        let s = binding.as_str();

        if let Some(postfix) = s.strip_prefix("approval") {
            let x = postfix.parse().map_err(|_| {
                anyhow!("Can't parse Q for approval, expected number, got: {postfix}")
            })?;
            return Ok(Self::Approval(x));
        }

        match s {
            "plurality" => Ok(Self::Plurality),
            "inverseplurality" => Ok(Self::InversePlurality),
            "borda" => Ok(Self::Borda),
            "black" => Ok(Self::Black),
            "copelandi" => Ok(Self::CopelandI),
            "copelandii" => Ok(Self::CopelandII),
            "copelandiii" => Ok(Self::CopelandIII),
            name => Err(anyhow!("Unknown rule name: {name}")),
        }
    }
}

#[derive(ValueEnum, Copy, Clone, Debug)]
pub enum InputFormat {
    Cvr,
}
