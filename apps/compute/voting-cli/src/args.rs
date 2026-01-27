use std::path::PathBuf;

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
    #[arg(short, long, value_enum)]
    pub rule: RuleName,
}

#[derive(Debug, Clone, Copy, ValueEnum)]
pub enum RuleName {
    /// Plurality rule.
    ///
    /// Candidates are chosen by the total number of first-place votes for them.
    Plurality,
    /// Approval rule with q=2.
    ///
    /// Candidates are chosen by the total number of votes in the first q places for each ballot.
    Approval2,
    /// Approval rule with q=3.
    ///
    /// Candidates are chosen by the total number of votes in the first q places for each ballot.
    Approval3,
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
    /// Simpson's rule.
    ///
    /// Candidates are scored based on their worst defeat in head-to-head. Best winning margin is chosen.
    Simpson,
    /// Minmax rule.
    /// Each candidate's score is the maximum margin of them against opponents.
    /// The candidate(-s) with the minimum score are winners.
    Minmax,
    /// Hare's rule.
    ///
    /// Choose a candidate with a majority of votes if they exist.
    /// Otherwise eliminate the candidate with the least amount of votes and continue.
    Hare,
    /// Nanson's rule.
    ///
    /// If there is a Borda winner, choose them. Otherwise eliminate candidates with below average Borda score.
    Nanson,
    /// Coombs' rule.
    ///
    /// If there is a candidate with a strict majority, choose them.
    /// Otherwise eliminate the candidate with the biggest number of last-place votes.
    Coombs,
    /// Inverse Borda rule.
    ///
    /// Each round eliminate the candidate with the lowest Borda score and repeat until one candidate is left.
    InverseBorda,
}

#[derive(ValueEnum, Copy, Clone, Debug)]
pub enum InputFormat {
    Cvr,
}
