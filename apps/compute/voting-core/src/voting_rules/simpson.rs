//! Simpson's voting rule type.

use crate::{
    decider::majority::MaxScoreDecider, scorer::simpson::SimpsonScorer,
    tie_breaker::fallthrough::FallthroughTieBreaker, voting_rules::voting_rule::VotingRule,
};

/// The Simpson's voting rule (aka Maxmin).
///
/// Each candidate's score is minimum pairwise victory margin against any opponent.
/// Then the candidate with the maximum score is the winner.
pub type SimpsonRule = SimpsonRuleWith<FallthroughTieBreaker>;

/// The Simpson's voting rule with a custom tie-breaker.
pub type SimpsonRuleWith<TB> = VotingRule<SimpsonScorer, MaxScoreDecider<isize>, TB>;
