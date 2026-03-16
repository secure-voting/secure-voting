//! Nanson's voting rule type.

use crate::{
    decider::maxscore::MaxScoreDecider,
    models::ranking::RankingBallot,
    scorer::borda::BordaScorer,
    tie_breaker::fallthrough::FallthroughTieBreaker,
    voting_rules::elimination::{
        criterion::below_average_score::BelowAverageElimination, rule::Elimination,
        stop::no_early_stop::NoEarlyStop,
    },
};

/// The Nanson's voting rule.
///
/// Compute scores via Borda.
/// If there is a single winner, the voting round ends with them a winner.
/// Else, eliminate all the candidates with below average scoring and repeat
pub type NansonRule = NansonRuleWith<FallthroughTieBreaker>;

/// The Nanson's voting rule type with a custom tie-breaker.
pub type NansonRuleWith<TB> = Elimination<
    BordaScorer<RankingBallot>,
    BelowAverageElimination,
    MaxScoreDecider<usize>,
    TB,
    RankingBallot,
    NoEarlyStop<RankingBallot>,
>;
