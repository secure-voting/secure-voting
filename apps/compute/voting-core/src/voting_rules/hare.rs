//! Hare's voting rule type.

use crate::{
    decider::maxscore::MaxScoreDecider,
    scorer::plurality::PluralityScorer,
    tie_breaker::fallthrough::FallthroughTieBreaker,
    voting_rules::elimination::{
        criterion::minscore::MinScoreElimination, rule::Elimination,
        stop::majority_stop::MajorityStop,
    },
};

/// The Hare's voting rule.
///
/// Do a regular plurality voting step.
/// If there is a candidate with a strict majority of votes, they are the winner.
/// Otherwise, eliminate the candidate with the least amount of votes and repeat until the winner is chosen.
pub type HareRule = HareRuleWith<FallthroughTieBreaker>;

/// The Hare's voting rule with a custom tie-breaker.
pub type HareRuleWith<TB> =
    Elimination<PluralityScorer, MinScoreElimination, MaxScoreDecider<usize>, TB, MajorityStop>;
