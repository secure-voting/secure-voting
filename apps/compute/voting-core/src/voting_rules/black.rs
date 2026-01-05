//! Black's voting rule implementation.

use crate::{
    decider::condorcet::CondorcetDecider,
    scorer::condorcet::CondorcetScorer,
    tie_breaker::fallthrough::FallthroughTieBreaker,
    voting_rules::{adaptors::fallback::Fallback, borda::BordaRule, voting_rule::VotingRule},
};

/// Helper Condorcet Voting rule type.
type CondorcetRule<TB = FallthroughTieBreaker> = VotingRule<CondorcetScorer, CondorcetDecider, TB>;

/// Black Voting rule type.
///
/// If there is a Condorcet Winner, choose them, otherwise use Borda as fallback.
pub type BlackRule<TB> = Fallback<CondorcetRule<TB>, BordaRule>;
