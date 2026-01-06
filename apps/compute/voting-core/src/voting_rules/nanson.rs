//! Nanson's voting rule type.

use crate::{
    decider::majority::MajorityDecider,
    scorer::borda::BordaScorer,
    tie_breaker::fallthrough::FallthroughTieBreaker,
    voting_rules::elimination::{
        below_average_score::BelowAverageElimination, rule::Elimination,
        stop::no_early_stop::NoEarlyStop,
    },
};

/// The Nanson's voting rule.
///
/// Compute scores via Borda.
/// If there is a single winner, the voting round ends with them a winner.
/// Else, eliminate all the candidates with below average scoring and repeat
pub type NansonRule<TB = FallthroughTieBreaker> =
    Elimination<BordaScorer, BelowAverageElimination, MajorityDecider, TB, NoEarlyStop>;
