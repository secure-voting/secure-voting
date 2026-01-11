//! Anti-plurality voting rule implementation.

use crate::{
    decider::minority::MinScoreDecider, scorer::anti_plurality::AntiPluralityScorer,
    tie_breaker::fallthrough::FallthroughTieBreaker, voting_rules::voting_rule::VotingRule,
};

/// Anti-plurality Voting rule type.
///
/// Each voter gives one point to everyone except the bottom candidate.
/// Winners are selected by the plurality among their score sums.
/// If there are several winners, the result is left undecided.
pub type AntiPluralityRule = AntiPluralityRuleWith<FallthroughTieBreaker>;

/// Anti-plurality Voting rule type with a custom tie-breaker.
pub type AntiPluralityRuleWith<TB> = VotingRule<AntiPluralityScorer, MinScoreDecider<usize>, TB>;
