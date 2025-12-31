use crate::{decider::Decider, scorer::Scorer, tie_breaker::TieBreaker, types::Profile};

pub struct VotingRule<S, D, T> {
    scorer: S,
    decider: D,
    tiebreaker: T,
}

impl<S, D, T> VotingRule<S, D, T>
where
    S: Scorer,
    D: Decider,
    T: TieBreaker,
{
    fn run(&self, profile: &Profile) -> usize {
        let scores = S::compute_score(profile);
        let candidates = D::decide(&scores);
        T::tie_break(&candidates, profile)
    }
}
