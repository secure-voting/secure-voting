//! Score voting rule implementation.

use crate::{
    decider::maxscore::MaxScoreDecider, models::scoring::ScoreBallot,
    scorer::scoring::ScoringScorer, tie_breaker::fallthrough::FallthroughTieBreaker,
    voting_rules::voting_rule::VotingRule,
};

/// Score Voting rule type.
///
/// Each voter gives a range of scores to each of candidates, then the scores are summed up.
/// The largest score wins.
pub type ScoreRule = ScoreRuleWith<FallthroughTieBreaker, ScoreBallot>;

/// Score Voting rule type with a custom tie-breaker.
pub type ScoreRuleWith<TB, Ballot> =
    VotingRule<ScoringScorer, MaxScoreDecider<usize>, TB, Ballot, usize>;
