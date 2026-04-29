//! Different methods of constructing a voting rule.
//!
//! This module defines the trait [`VotingRuleExec`] and pre-defined voting rule implementations.

use std::fmt::Debug;

use bon::Builder;

use crate::{models::profile::Profile, prelude::CandidateId, tie_breaker::RuleOutcome};

pub mod adaptors;
pub mod elimination;
pub mod voting_rule;

pub mod anti_plurality;
pub mod approval;
pub mod black;
pub mod borda;
pub mod coombs;
pub mod copeland;
pub mod hare;
pub mod inverse_borda;
pub mod minmax;
pub mod nanson;
pub mod plurality;
pub mod practical_condorcet;
pub mod q_paretian;
pub mod simpson;
pub mod threshold;

#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
#[derive(Debug, Clone, Default, Builder)]
/// Summary of the election results.
pub struct Summary {
    /// Total number of ballots in the election.
    pub total_ballots: usize,
    /// Number of valid ballots.
    pub valid_ballots: usize,
    /// Number of invalid ballots.
    pub invalid_ballots: usize,
    /// Number of candidates in the election.
    pub candidates_count: usize,
    /// Number of winners selected.
    pub winner_count: usize,
    /// Size of the committee.
    pub committee_size: usize,
    /// Number of rounds in the election.
    pub rounds_count: usize,
    /// Whether a tie was detected in the results.
    pub tie_detected: bool,
}

#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
#[derive(Debug, Clone, Default, Builder)]
/// Numeric scores from the election.
pub struct Numeric {
    /// Score of the winner.
    pub winner_score: f64,
    /// Score of the runner-up.
    pub runner_up_score: f64,
    /// Margin between winner and runner-up.
    pub margin: f64,
    /// Normalized margin (margin / total_ballots).
    pub normalized_margin: f64,
    /// Average score across candidates.
    pub average_score: f64,
}

#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
#[cfg_attr(feature = "serde", serde(tag = "score_kind"))]
#[derive(Debug, Clone)]
/// Score representation for candidates.
pub enum Score {
    /// A single scalar value score.
    #[cfg_attr(feature = "serde", serde(rename = "scalar"))]
    Scalar {
        /// ID of the candidate.
        candidate_id: String,
        /// Name of the candidate.
        candidate_name: String,
        /// Numeric value of the score.
        value: f64,
    },

    /// A vector of values representing multiple scores.
    #[cfg_attr(feature = "serde", serde(rename = "vector"))]
    Vector {
        /// ID of the candidate.
        candidate_id: String,
        /// Name of the candidate.
        candidate_name: String,
        /// Vector of numeric values.
        values: Vec<f64>,
    },
}

#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
#[derive(Debug, Clone, Default, Builder)]
/// Size of each round in the election.
pub struct RoundSize {
    /// Round number.
    round: usize,
    /// Number of remaining candidates.
    remaining_candidates: usize,
}

#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
#[derive(Debug, Clone, Default, Builder)]
/// Series of scores over multiple rounds.
pub struct Series {
    /// Final scores of candidates after all rounds.
    #[cfg_attr(feature = "serde", serde(skip_serializing_if = "Option::is_none"))]
    candidate_scores_final: Option<Vec<Score>>,
    /// Sizes of each round.
    #[cfg_attr(feature = "serde", serde(skip_serializing_if = "Option::is_none"))]
    round_sizes: Option<Vec<RoundSize>>,
}

#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
#[derive(Debug, Clone, Default, Builder)]
/// Metrics from the election execution.
pub struct Metrics {
    /// Summary of the election.
    summary: Summary,
    /// Numeric details if available.
    #[cfg_attr(feature = "serde", serde(skip_serializing_if = "Option::is_none"))]
    numeric: Option<Numeric>,
    /// Series data if available.
    #[cfg_attr(feature = "serde", serde(skip_serializing_if = "Option::is_none"))]
    series: Option<Series>,
}

#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
#[cfg_attr(feature = "serde", serde(rename_all = "snake_case"))]
#[derive(Debug, Clone, Default)]
/// Kind of voting rule executed.
pub enum Kind {
    /// Single-step voting rule.
    #[default]
    SingleStep,
    /// Elimination rounds voting rule.
    EliminationRounds,
    /// Selection rounds voting rule.
    SelectionRounds,
    /// Pairwise comparison voting rule.
    PairwiseComparison,
    /// Custom voting rule.
    Custom,
}

#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
#[derive(Debug, Clone, Default, Builder)]
/// Final result of the voting rule.
pub struct Final {
    /// IDs of the winning candidates.
    winner_ids: Vec<String>,
}

#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
#[derive(Debug, Clone, Default, Builder)]
/// Individual step in the voting protocol.
pub struct Step {
    /// Step number.
    step: usize,
    /// Title of the step.
    title: String,
    /// Action performed in this step.
    action: String,
    /// Remaining candidate IDs after this step.
    #[cfg_attr(feature = "serde", serde(skip_serializing_if = "Option::is_none"))]
    remaining_candidate_ids: Option<Vec<String>>,
    /// Selected candidate IDs in this step.
    #[cfg_attr(feature = "serde", serde(skip_serializing_if = "Option::is_none"))]
    selected_candidate_ids: Option<Vec<String>>,
    #[cfg_attr(feature = "serde", serde(skip_serializing_if = "Option::is_none"))]
    eliminated_candidate_ids: Option<Vec<String>>,
    #[cfg_attr(feature = "serde", serde(skip_serializing_if = "Option::is_none"))]
    scores: Option<Vec<Score>>,
    note: Option<String>,
}

#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
#[derive(Debug, Clone, Default, Builder)]
/// Protocol of the voting rule execution.
pub struct Protocol {
    /// Kind of voting rule.
    kind: Kind,
    /// Steps executed in the protocol.
    steps: Vec<Step>,
    /// Final result.
    #[cfg_attr(feature = "serde", serde(rename = "final"))]
    r#final: Final,
}

impl Step {
    /// Set remaining candidates for this step.
    pub fn set_remaining(&mut self, cands: &[CandidateId]) {
        self.remaining_candidate_ids = Some(cands.iter().map(ToString::to_string).collect());
    }

    /// Set eliminated candidates for this step.
    pub fn set_eliminated(&mut self, cands: &[CandidateId]) {
        self.eliminated_candidate_ids = Some(cands.iter().map(ToString::to_string).collect());
    }

    /// Set the action description for this step.
    pub fn set_action(&mut self, action: &str) {
        self.action = action.to_owned();
    }
}

impl Protocol {
    /// Add a step to the protocol.
    pub fn add_step(&mut self, mut step: Step) {
        step.step = self.steps.len() + 1;
        self.steps.push(step);
    }

    /// Add multiple steps to the protocol.
    pub fn add_steps(&mut self, steps: &[Step]) {
        let orig_len = self.steps.len();
        self.steps.extend(steps.iter().cloned());

        self.steps[orig_len..]
            .iter_mut()
            .for_each(|x| x.step += orig_len);
    }
}

/// Trait for converting types to score representations.
pub trait ToScore {
    /// Convert to a score representation.
    fn to_score(&self, cand_id: String, cand_name: String) -> Score;
}

impl ToScore for usize {
    fn to_score(&self, cand_id: String, cand_name: String) -> Score {
        Score::Scalar {
            candidate_id: cand_id,
            candidate_name: cand_name,
            value: *self as f64,
        }
    }
}

impl ToScore for isize {
    fn to_score(&self, cand_id: String, cand_name: String) -> Score {
        Score::Scalar {
            candidate_id: cand_id,
            candidate_name: cand_name,
            value: *self as f64,
        }
    }
}

impl ToScore for (usize, usize) {
    fn to_score(&self, cand_id: String, cand_name: String) -> Score {
        Score::Vector {
            candidate_id: cand_id,
            candidate_name: cand_name,
            values: vec![self.0 as f64, self.1 as f64],
        }
    }
}

impl ToScore for Vec<usize> {
    fn to_score(&self, cand_id: String, cand_name: String) -> Score {
        Score::Vector {
            candidate_id: cand_id,
            candidate_name: cand_name,
            values: self.iter().map(|&x| x as f64).collect(),
        }
    }
}

impl ToScore for Vec<bool> {
    fn to_score(&self, cand_id: String, cand_name: String) -> Score {
        Score::Vector {
            candidate_id: cand_id,
            candidate_name: cand_name,
            values: self.iter().map(|&x| if x { 1.0 } else { 0.0 }).collect(),
        }
    }
}

impl ToScore for f64 {
    fn to_score(&self, cand_id: String, cand_name: String) -> Score {
        Score::Scalar {
            candidate_id: cand_id,
            candidate_name: cand_name,
            value: *self,
        }
    }
}

/// Trait for all the voting rules, simple and complex ones.
pub trait VotingRuleExec<Ballot> {
    /// Returned if the voting pipeline can't be completed.
    type Error: Debug;

    /// Run the constructed pipeline.
    ///
    /// # Errors
    ///
    /// Returns an error if any of voting steps failed.
    /// Usually a sum type of the steps' error types.
    fn execute(
        &self,
        profile: &Profile<Ballot>,
    ) -> Result<(RuleOutcome, Metrics, Protocol), Self::Error>;

    /// Constructor-like method to get new instances.
    fn create_default() -> Self
    where
        Self: Sized;
}
