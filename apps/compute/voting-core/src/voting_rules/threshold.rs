//! Threshold voting rule type.

use crate::{
    decider::threshold::ThresholdDecider, models::ranking::RankingBallot,
    scorer::threshold::ThresholdScorer, tie_breaker::fallthrough::FallthroughTieBreaker,
    voting_rules::voting_rule::VotingRule,
};

/// Threshold voting rule.
///
/// Each candidate gets a vector associated to it.
/// The `i`-th vector entry represents how many times the chosen candidate got ranked on the `i`-th place.
pub type ThresholdRule = ThresholdRuleWith<FallthroughTieBreaker, RankingBallot>;

/// The Simpson's voting rule with a custom tie-breaker.
pub type ThresholdRuleWith<TB, Ballot> =
    VotingRule<ThresholdScorer<RankingBallot>, ThresholdDecider, TB, Ballot>;
