//! Anti-plurality voting rule implementation.

use crate::{
    decider::majority::MajorityDecider, scorer::anti_plurality::AntiPluralityScorer,
    tie_breaker::fallthrough::FallthroughTieBreaker, voting_rules::voting_rule::VotingRule,
};

/// Anti-plurality Voting rule type.
///
/// Each voter gives one point to everyone except the bottom candidate.
/// Winners are selected by the plurality among their score sums.
/// If there are several winners, the result is left undecided.
pub type AntiPluralityRule =
    VotingRule<AntiPluralityScorer, MajorityDecider, FallthroughTieBreaker>;
