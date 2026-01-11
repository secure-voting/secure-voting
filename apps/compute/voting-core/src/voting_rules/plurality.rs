//! Plurality voting rule implementation.

use crate::{
    decider::majority::MaxScoreDecider, scorer::plurality::PluralityScorer,
    tie_breaker::fallthrough::FallthroughTieBreaker, voting_rules::voting_rule::VotingRule,
};

/// Plurality Voting rule type.
///
/// Each voter gives one point to their top candidate.
/// Winners are selected by the plurality among their score sums.
/// If there are several winners, the result is left undecided.
pub type PluralityRule = PluralityRuleWith<FallthroughTieBreaker>;

/// Plurality Voting rule type with a custom tie-breaker.
pub type PluralityRuleWith<TB> = VotingRule<PluralityScorer, MaxScoreDecider<usize>, TB>;
