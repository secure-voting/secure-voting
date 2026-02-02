//! Condorcet Practical voting rule type.

use crate::{
    decider::condorcet::CondorcetDecider, prelude::fallback::Fallback,
    scorer::condorcet::CondorcetScorer, voting_rules::voting_rule::VotingRule,
};

/// Condorcet Practical voting rule.
///
/// If there exists a Condorcet winner, choose them.
/// Otherwise fallback to another rule to decide.
pub type CondorcetPracticalRule<FB, TB, Ballot> =
    Fallback<VotingRule<CondorcetScorer, CondorcetDecider, TB, Ballot>, FB, Ballot>;
