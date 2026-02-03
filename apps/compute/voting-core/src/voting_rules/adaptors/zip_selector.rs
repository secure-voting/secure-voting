//! `ZipSelector` module.

use std::marker::PhantomData;

use crate::{
    decider::Decider,
    prelude::{CandidateId, EliminationCriterion, EliminationStopCondition, Profile, RuleOutcome},
    scorer::Score,
};

/// Wrapper over an action to decide which part of the tuple to act on.
#[derive(Debug, Clone, Copy)]
pub struct ZipSelector<const I: usize, A, T1, T2> {
    /// Action to perform on a tuple's element.
    action: A,

    /// `PhantomData` type marker for the 2 input types of a tuple.
    _marker: PhantomData<(T1, T2)>,
}

impl<A: Decider<Input = T1>, T1: Clone, T2> Decider for ZipSelector<0, A, T1, T2> {
    type Input = (T1, T2);

    type Error = A::Error;

    fn decide(&self, scores: &Score<Self::Input>) -> Result<Vec<CandidateId>, Self::Error> {
        let new_score = scores.score().0.clone();
        let new_candidates = scores.candidates();

        self.action.decide(&Score::new(new_score, new_candidates))
    }

    fn new() -> Self {
        Self {
            action: A::new(),
            _marker: PhantomData,
        }
    }
}

impl<A: Decider<Input = T2>, T1, T2: Clone> Decider for ZipSelector<1, A, T1, T2> {
    type Input = (T1, T2);

    type Error = A::Error;

    fn decide(&self, scores: &Score<Self::Input>) -> Result<Vec<CandidateId>, Self::Error> {
        let new_score = scores.score().1.clone();
        let new_candidates = scores.candidates();

        self.action.decide(&Score::new(new_score, new_candidates))
    }

    fn new() -> Self {
        Self {
            action: A::new(),
            _marker: PhantomData,
        }
    }
}

impl<A: EliminationCriterion<Score = T1>, T1: Clone, T2> EliminationCriterion
    for ZipSelector<0, A, T1, T2>
{
    type Score = (T1, T2);

    fn eliminate(&self, scores: &Score<Self::Score>) -> Vec<CandidateId> {
        let new_score = scores.score().0.clone();
        let new_candidates = scores.candidates();

        self.action
            .eliminate(&Score::new(new_score, new_candidates))
    }

    fn new() -> Self {
        Self {
            action: A::new(),
            _marker: PhantomData,
        }
    }
}

impl<A: EliminationCriterion<Score = T2>, T1, T2: Clone> EliminationCriterion
    for ZipSelector<1, A, T1, T2>
{
    type Score = (T1, T2);

    fn eliminate(&self, scores: &Score<Self::Score>) -> Vec<CandidateId> {
        let new_score = scores.score().1.clone();
        let new_candidates = scores.candidates();

        self.action
            .eliminate(&Score::new(new_score, new_candidates))
    }

    fn new() -> Self {
        Self {
            action: A::new(),
            _marker: PhantomData,
        }
    }
}

impl<Ballot, A: EliminationStopCondition<T1, Ballot>, T1: Clone, T2>
    EliminationStopCondition<(T1, T2), Ballot> for ZipSelector<0, A, T1, T2>
{
    fn should_stop(
        &self,
        scores: &Score<(T1, T2)>,
        outcome: &RuleOutcome,
        profile: &Profile<Ballot>,
    ) -> bool {
        let new_score = scores.score().0.clone();
        let new_candidates = scores.candidates();

        self.action
            .should_stop(&Score::new(new_score, new_candidates), outcome, profile)
    }

    fn new() -> Self {
        Self {
            action: A::new(),
            _marker: PhantomData,
        }
    }
}

impl<Ballot, A: EliminationStopCondition<T2, Ballot>, T1, T2: Clone>
    EliminationStopCondition<(T1, T2), Ballot> for ZipSelector<1, A, T1, T2>
{
    fn should_stop(
        &self,
        scores: &Score<(T1, T2)>,
        outcome: &RuleOutcome,
        profile: &Profile<Ballot>,
    ) -> bool {
        let new_score = scores.score().1.clone();
        let new_candidates = scores.candidates();

        self.action
            .should_stop(&Score::new(new_score, new_candidates), outcome, profile)
    }

    fn new() -> Self {
        Self {
            action: A::new(),
            _marker: PhantomData,
        }
    }
}
