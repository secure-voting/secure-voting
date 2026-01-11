//! Coombs' voting rule type.

use crate::{
    decider::majority::MaxScoreDecider,
    prelude::zip_selector::ZipSelector,
    scorer::{anti_plurality::AntiPluralityScorer, plurality::PluralityScorer, zip::ZipScorer},
    tie_breaker::fallthrough::FallthroughTieBreaker,
    voting_rules::elimination::{
        criterion::maxscore::MaxScoreElimination, rule::Elimination,
        stop::majority_stop::MajorityStop,
    },
};

/// The Coombs' voting rule.
///
/// Do a regular antiplurality voting step.
/// If there is a candidate with a strict majority of votes, they are the winner.
/// Otherwise, eliminate the candidate with the *most* last-place votes and repeat until the winner is chosen.
pub type CoombsRule = CoombsRuleWith<FallthroughTieBreaker>;

/// The Coombs' voting rule type with a custom tie-breaker.
pub type CoombsRuleWith<TB> = Elimination<
    ZipScorer<PluralityScorer, AntiPluralityScorer>,
    ZipSelector<1, MaxScoreElimination, Vec<usize>, Vec<usize>>,
    ZipSelector<0, MaxScoreDecider<usize>, Vec<usize>, Vec<usize>>,
    TB,
    ZipSelector<0, MajorityStop, Vec<usize>, Vec<usize>>,
>;
