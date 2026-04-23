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
pub struct Summary {
    total_ballots: usize,
    valid_ballots: usize,
    invalid_ballots: usize,
    candidates_count: usize,
    winner_count: usize,
    committee_size: usize,
    rounds_count: usize,
}

#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
#[derive(Debug, Clone, Default, Builder)]
pub struct Numeric {
    winner_score: f64,
    runner_up_score: f64,
    margin: f64,
    average_score: f64,
}

#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
#[derive(Debug, Clone, Default, Builder)]
pub struct Score {
    candidate_id: String,
    candidate_name: String,
    value: f64,
}

#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
#[derive(Debug, Clone, Default, Builder)]
pub struct RoundSize {
    round: usize,
    remaining_candidates: usize,
}

#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
#[derive(Debug, Clone, Default, Builder)]
pub struct Series {
    candidate_scores_final: Option<Vec<Score>>,
    round_sizes: Option<Vec<RoundSize>>,
}

#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
#[derive(Debug, Clone, Default, Builder)]
pub struct Metrics {
    summary: Summary,
    numeric: Option<Numeric>,
    series: Option<Series>,
}

#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
#[derive(Debug, Clone, Default)]
pub enum Kind {
    #[default]
    SingleStep,
    EliminationRounds,
    SelectionRounds,
    PairwiseComparison,
    Custom,
}

#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
#[derive(Debug, Clone, Default, Builder)]
pub struct Final {
    winner_ids: Vec<String>,
}

#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
#[derive(Debug, Clone, Default, Builder)]
pub struct Step {
    step: usize,
    title: String,
    action: String,
    remaining_candidate_ids: Option<Vec<String>>,
    selected_candidate_ids: Option<Vec<String>>,
    eliminated_candidate_ids: Option<Vec<String>>,
    scores: Option<Vec<Score>>,
    note: Option<String>,
}

#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
#[derive(Debug, Clone, Default, Builder)]
pub struct Protocol {
    kind: Kind,
    steps: Vec<Step>,
    r#final: Final,
}

impl Step {
    pub fn set_remaining(&mut self, cands: &[CandidateId]) {
        self.remaining_candidate_ids = Some(cands.iter().map(ToString::to_string).collect());
    }

    pub fn set_eliminated(&mut self, cands: &[CandidateId]) {
        self.eliminated_candidate_ids = Some(cands.iter().map(ToString::to_string).collect());
    }

    pub fn set_action(&mut self, action: &str) {
        self.action = action.to_owned();
    }
}

impl Protocol {
    pub fn add_step(&mut self, step: Step) {
        self.steps.push(step);
    }

    pub fn add_steps(&mut self, steps: &[Step]) {
        let orig_len = self.steps.len();
        self.steps.extend(steps.iter().cloned());

        self.steps[orig_len..]
            .iter_mut()
            .for_each(|x| x.step += orig_len);
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
