//! Condorcet Practical voting rule type.

use crate::{
    decider::condorcet::CondorcetDecider, prelude::fallback::Fallback,
    scorer::condorcet::CondorcetScorer, tie_breaker::fallthrough::FallthroughTieBreaker,
    voting_rules::voting_rule::VotingRule,
};

/// Condorcet Practical voting rule.
///
/// If there exists a Condorcet winner, choose them.
/// Otherwise fallback to another rule to decide.
pub type CondorcetPracticalRule<FB, TB = FallthroughTieBreaker> =
    Fallback<VotingRule<CondorcetScorer, CondorcetDecider, TB>, FB>;
