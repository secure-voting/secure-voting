//! Borda voting rule implementation.

use crate::{
    decider::majority::MajorityDecider, scorer::borda::BordaScorer,
    tie_breaker::fallthrough::FallthroughTieBreaker, voting_rules::voting_rule::VotingRule,
};

/// Borda Voting rule type.
///
/// Each voter gives point to candidate depending on their position:
/// first place gets most points, second place gets one less and so on until the last place gets 0.
/// Winners are selected by the plurality among their score sums.
/// If there are several winners, the result is left undecided.
pub type BordaRule<TB = FallthroughTieBreaker> = VotingRule<BordaScorer, MajorityDecider, TB>;
