//! Black's voting rule implementation.

use crate::{
    decider::condorcet::CondorcetDecider,
    models::ranking::RankingBallot,
    scorer::condorcet::CondorcetScorer,
    tie_breaker::fallthrough::FallthroughTieBreaker,
    voting_rules::{adaptors::fallback::Fallback, borda::BordaRule, voting_rule::VotingRule},
};

/// Helper Condorcet Voting rule type.
type CondorcetRule<Ballot, TB = FallthroughTieBreaker> =
    VotingRule<CondorcetScorer, CondorcetDecider, TB, Ballot>;

/// Black Voting rule type.
///
/// If there is a Condorcet Winner, choose them, otherwise use Borda as fallback.
pub type BlackRule = BlackRuleWith<FallthroughTieBreaker, RankingBallot>;

/// Black Voting rule type with a custom tie-breaker.
pub type BlackRuleWith<TB, Ballot> = Fallback<CondorcetRule<Ballot, TB>, BordaRule, Ballot>;
