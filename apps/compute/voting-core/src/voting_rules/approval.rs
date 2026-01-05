//! Approval voting rule implementation.

use crate::{
    decider::majority::MajorityDecider, scorer::approval::ApprovalScorer,
    tie_breaker::fallthrough::FallthroughTieBreaker, voting_rules::voting_rule::VotingRule,
};

/// Q-Approval Voting rule type.
///
/// Each voter gives one point to their top `Q` candidates.
/// Winners are selected using plurality over the aggregated approval scores.
/// If multiple winners remain, the result is left undecided.
pub type ApprovalRule<const Q: usize, TB = FallthroughTieBreaker> =
    VotingRule<ApprovalScorer<Q>, MajorityDecider, TB>;
