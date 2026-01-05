//! Copeland's family of voting rules' implementation.

use crate::{
    decider::majority::MajorityDecider,
    scorer::copeland::{
        copeland_i::CopelandIScorer, copeland_ii::CopelandIIScorer, copeland_iii::CopelandIIIScorer,
    },
    tie_breaker::fallthrough::FallthroughTieBreaker,
    voting_rules::voting_rule::VotingRule,
};

/// Copeland I Voting rule type.
///
/// Each candidate's score is the number of strict wins against
/// other candidates in head-to-head.
pub type CopelandIRule<TB = FallthroughTieBreaker> =
    VotingRule<CopelandIScorer, MajorityDecider, TB>;

/// Copeland II Voting rule type.
///
/// Each candidate's score is the difference between
/// the number of strict wins and strict losses in head-to-head comparisons.
pub type CopelandIIRule<TB = FallthroughTieBreaker> =
    VotingRule<CopelandIIScorer, MajorityDecider, TB>;

/// Copeland III Voting rule type.
///
/// Each candidate's score is the total margin of win in head-to-head comparisons.
pub type CopelandIIIRule<TB = FallthroughTieBreaker> =
    VotingRule<CopelandIIIScorer, MajorityDecider, TB>;
