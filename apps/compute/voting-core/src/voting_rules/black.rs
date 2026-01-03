//! Black's voting rule implementation.

use crate::{
    decider::condorcet::CondorcetDecider,
    prelude::BordaRule,
    scorer::condorcet::CondorcetScorer,
    tie_breaker::fallthrough::FallthroughTieBreaker,
    voting_rules::{adaptors::fallback::Fallback, voting_rule::VotingRule},
};

/// Helper Condorcet Voting rule type.
type CondorcetRule = VotingRule<CondorcetScorer, CondorcetDecider, FallthroughTieBreaker>;

/// Black Voting rule type.
///
/// If there is a Condorcet Winner, choose them, otherwise use Borda as fallback.
pub type BlackRule = Fallback<CondorcetRule, BordaRule>;
