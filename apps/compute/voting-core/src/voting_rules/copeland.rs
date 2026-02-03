//! Copeland's family of voting rules' implementation.

use crate::{
    decider::maxscore::MaxScoreDecider,
    models::ranking::RankingBallot,
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
pub type CopelandIRule = CopelandIRuleWith<FallthroughTieBreaker, RankingBallot>;

/// Copeland I Voting rule type with a custom tie-breaker.
pub type CopelandIRuleWith<TB, Ballot> =
    VotingRule<CopelandIScorer, MaxScoreDecider<usize>, TB, Ballot>;

/// Copeland II Voting rule type.
///
/// Each candidate's score is the difference between
/// the number of strict wins and strict losses in head-to-head comparisons.
pub type CopelandIIRule = CopelandIIRuleWith<FallthroughTieBreaker, RankingBallot>;

/// Copeland I Voting rule type with a custom tie-breaker.
pub type CopelandIIRuleWith<TB, Ballot> =
    VotingRule<CopelandIIScorer, MaxScoreDecider<isize>, TB, Ballot>;

/// Copeland III Voting rule type.
///
/// Each candidate's score is the total margin of win in head-to-head comparisons.
pub type CopelandIIIRule = CopelandIIIRuleWith<FallthroughTieBreaker, RankingBallot>;

/// Copeland I Voting rule type with a custom tie-breaker.
pub type CopelandIIIRuleWith<TB, Ballot> =
    VotingRule<CopelandIIIScorer, MaxScoreDecider<isize>, TB, Ballot>;
