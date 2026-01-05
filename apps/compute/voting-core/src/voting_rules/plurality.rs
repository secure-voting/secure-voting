//! Plurality voting rule implementation.

use crate::{
    decider::majority::MajorityDecider, scorer::plurality::PluralityScorer,
    tie_breaker::fallthrough::FallthroughTieBreaker, voting_rules::voting_rule::VotingRule,
};

/// Plurality Voting rule type.
///
/// Each voter gives one point to their top candidate.
/// Winners are selected by the plurality among their score sums.
/// If there are several winners, the result is left undecided.
pub type PluralityRule<TB = FallthroughTieBreaker> =
    VotingRule<PluralityScorer, MajorityDecider, TB>;
