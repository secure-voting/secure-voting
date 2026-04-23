//! Simpson's voting rule type.

use crate::{
    decider::maxscore::MaxScoreDecider, models::ranking::RankingBallot,
    scorer::simpson::SimpsonScorer, tie_breaker::fallthrough::FallthroughTieBreaker,
    voting_rules::voting_rule::VotingRule,
};

/// The Simpson's voting rule (aka Maxmin).
///
/// Each candidate's score is minimum pairwise victory margin against any opponent.
/// Then the candidate with the maximum score is the winner.
pub type SimpsonRule = SimpsonRuleWith<FallthroughTieBreaker, RankingBallot>;

/// The Simpson's voting rule with a custom tie-breaker.
pub type SimpsonRuleWith<TB, Ballot> =
    VotingRule<SimpsonScorer, MaxScoreDecider<isize>, TB, Ballot, isize>;
