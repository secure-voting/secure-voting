use std::fmt::Debug;

use crate::{decider::Decider, profile::Profile, scorer::Scorer, tie_breaker::TieBreaker};
use thiserror::Error;

#[derive(Error, Debug)]
pub enum VotingRuleError<SE, DE, TE>
where
    SE: Debug,
    DE: Debug,
    TE: Debug,
{
    #[error(transparent)]
    ScoringError(SE),
    #[error(transparent)]
    DecisionError(DE),
    #[error(transparent)]
    TieBreakError(TE),
}

pub struct VotingRule<S: Scorer, D: Decider, T: TieBreaker> {
    scorer: S,
    decider: D,
    tiebreaker: T,
}

type VotingRuleResult<S, D, T> = Result<
    usize,
    VotingRuleError<<S as Scorer>::Error, <D as Decider>::Error, <T as TieBreaker>::Error>,
>;

impl<S, D, T> VotingRule<S, D, T>
where
    S: Scorer<Output = D::Input>,
    D: Decider,
    T: TieBreaker,
    <S as Scorer>::Error: Debug,
    <D as Decider>::Error: Debug,
    <T as TieBreaker>::Error: Debug,
{
    pub fn new(scorer: S, decider: D, tiebreaker: T) -> Self {
        Self {
            scorer,
            decider,
            tiebreaker,
        }
    }

    pub fn run(&self, profile: &Profile) -> VotingRuleResult<S, D, T> {
        let scores = self
            .scorer
            .compute_score(profile)
            .map_err(VotingRuleError::ScoringError)?;
        let candidates = self
            .decider
            .decide(&scores)
            .map_err(VotingRuleError::DecisionError)?;
        self.tiebreaker
            .tie_break(&candidates, profile)
            .map_err(VotingRuleError::TieBreakError)
    }
}
