//! Plurality voting rule implementaiton.

use crate::{
    decider::plurality::PluralityDecider, scorer::plurality::PluralityScorer,
    tie_breaker::fallthrough::FallthroughTieBreaker, voting_rules::voting_rule::VotingRule,
};

/// Plurality Voting rule type.
pub type PluralityRule = VotingRule<PluralityScorer, PluralityDecider, FallthroughTieBreaker>;
