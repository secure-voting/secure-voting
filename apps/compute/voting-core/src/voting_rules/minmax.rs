//! Minmax voting rule type.

use crate::{
    decider::minority::MinScoreDecider, scorer::minmax::MinmaxScorer,
    tie_breaker::fallthrough::FallthroughTieBreaker, voting_rules::voting_rule::VotingRule,
};

/// The minmax voting rule.
///
/// Each candidate's score is the maximum margin of them against opponents.
/// The candidate(-s) with the minimum score are winners.
pub type MinmaxRule = MinmaxRuleWith<FallthroughTieBreaker>;

/// The minmax voting rule type with a custom tie-breaker.
pub type MinmaxRuleWith<TB> = VotingRule<MinmaxScorer, MinScoreDecider<isize>, TB>;
