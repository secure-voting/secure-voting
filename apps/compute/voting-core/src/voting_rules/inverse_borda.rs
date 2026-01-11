//! Inverse borda voting rule module.

use crate::{
    decider::majority::MaxScoreDecider,
    prelude::Elimination,
    scorer::borda::BordaScorer,
    tie_breaker::fallthrough::FallthroughTieBreaker,
    voting_rules::elimination::{
        criterion::minscore::MinScoreElimination, stop::no_early_stop::NoEarlyStop,
    },
};

/// Inverse Borda's rule.
///
/// Calculate the Borda's count for each candidate.
/// Eliminate the candidate with the lowest score and repeat
/// until a unique winner can be elected.
pub type InverseBordaRule = InverseBordaRuleWith<FallthroughTieBreaker>;

/// Inverse Borda's rule with a custom tie-breaker.
pub type InverseBordaRuleWith<TB> =
    Elimination<BordaScorer, MinScoreElimination, MaxScoreDecider<usize>, TB, NoEarlyStop>;
