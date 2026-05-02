//! Fallback adaptor module.

use std::{fmt::Debug, marker::PhantomData};
use thiserror::Error;
use tracing::instrument;

use crate::{
    prelude::Profile,
    tie_breaker::RuleOutcome,
    voting_rules::{Kind, Metrics, Protocol, Step, VotingRuleExec},
};

/// A fallback adaptor.
///
/// If the primary rule can't decide a single winner, a fallback rule will be used to determine the winner instead.
#[derive(Debug, Clone, Copy)]
#[allow(clippy::struct_field_names)]
pub struct Fallback<R1, R2, Ballot> {
    /// Primary voting Rule
    primary: R1,
    /// Fallback voting Rule
    fallback: R2,
    /// Ballot type marker.
    _ballot_type: PhantomData<Ballot>,
}

impl<R1, R2, Ballot> Fallback<R1, R2, Ballot> {
    /// Construct a Fallback adaptor from the primary and fallback rules.
    pub fn new(primary: R1, fallback: R2) -> Self {
        Self {
            primary,
            fallback,
            _ballot_type: PhantomData,
        }
    }
}

/// Fallback adaptor error.
///
/// Has two variants:
///
/// - Primary
/// - Fallback
///
/// Variants represent which part of execution errored out.
#[derive(Error, Debug)]
pub enum FallbackError<P: Debug, F: Debug> {
    /// Returned when the primary voting rule returns an error itself.
    #[error("primary rule failed: {0:?}")]
    PrimaryError(P),
    /// Returned when the fallback voting rule returns an error itself.
    #[error("fallback rule failed: {0:?}")]
    FallbackError(F),
}

impl<R1, R2, Ballot> VotingRuleExec<Ballot> for Fallback<R1, R2, Ballot>
where
    R1: VotingRuleExec<Ballot>,
    R2: VotingRuleExec<Ballot>,
{
    type Error = FallbackError<R1::Error, R2::Error>;

    #[instrument(skip(self, profile))]
    fn execute(
        &self,
        profile: &Profile<Ballot>,
    ) -> Result<(RuleOutcome, Metrics, Protocol), Self::Error> {
        match self.primary.execute(profile) {
            Ok(outcome @ (RuleOutcome::UniqueWinner(_), _, _)) => {
                tracing::debug!("Primary rule returned a unique winner");
                let mut protocol = outcome.2;
                protocol.kind = Kind::PairwiseComparison;
                let mut step = Step::builder()
                    .step(1)
                    .title("Primary rule".into())
                    .action("primary_succeeded".into())
                    .build();
                step.set_remaining(&outcome.0.candidates());
                protocol.add_step(step);
                Ok((outcome.0, outcome.1, protocol))
            }
            Ok((RuleOutcome::MultipleWinners(_), _, _)) => {
                tracing::debug!("Primary rule can't decide winner, running fallback");
                let mut outcome = self
                    .fallback
                    .execute(profile)
                    .map_err(FallbackError::FallbackError)?;
                outcome.2.kind = Kind::PairwiseComparison;
                let mut step = Step::builder()
                    .step(outcome.2.steps.len() + 1)
                    .title("Primary rule".into())
                    .action("primary_tied".into())
                    .build();
                step.set_remaining(&outcome.0.candidates());
                outcome.2.add_step(step);
                Ok(outcome)
            }
            Err(e) => Err(FallbackError::PrimaryError(e)),
        }
    }

    fn create_default() -> Self
    where
        Self: Sized,
    {
        Fallback::default()
    }
}

impl<R1, R2, Ballot> Default for Fallback<R1, R2, Ballot>
where
    R1: VotingRuleExec<Ballot>,
    R2: VotingRuleExec<Ballot>,
{
    fn default() -> Self {
        Self {
            primary: R1::create_default(),
            fallback: R2::create_default(),
            _ballot_type: PhantomData,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::{
        models::BallotData,
        models::{candidate_id::CandidateId, profile::Profile, ranking::RankingBallot},
        tie_breaker::RuleOutcome,
        voting_rules::VotingRuleExec,
    };
    use mockall::mock;

    mock! {
        pub VotingRule {}

        impl VotingRuleExec<RankingBallot> for VotingRule {
            type Error = &'static str;

            fn execute(&self, profile: &Profile<RankingBallot>) -> Result<(RuleOutcome, Metrics, Protocol), <Self as VotingRuleExec<RankingBallot>>::Error>;
            fn create_default() -> Self where Self: Sized;
        }
    }
    fn fake_profile() -> Profile<RankingBallot> {
        let ballots: Vec<BallotData> = vec![BallotData::Simple(vec![
            CandidateId::new(0, "A"),
            CandidateId::new(1, "B"),
        ])];
        Profile::try_from((ballots, vec!["A".into(), "B".into()]))
            .expect("Profile is constructed incorrectly, revise test example")
    }
    #[test]
    fn primary_unique_winner_short_circuits() {
        let mut primary = MockVotingRule::new();
        let mut fallback = MockVotingRule::new();

        primary.expect_execute().times(1).return_const(Ok((
            RuleOutcome::UniqueWinner(CandidateId::new(0, "A")),
            Metrics::default(),
            Protocol::default(),
        )));

        fallback.expect_execute().times(0);

        let rule = Fallback::new(primary, fallback);

        let result = rule.execute(&fake_profile());

        assert_eq!(
            result.expect("The rule shouldn't fail").0,
            RuleOutcome::UniqueWinner(CandidateId::new(0, "A"))
        );
    }
    #[test]
    fn primary_multiple_winners_triggers_fallback() {
        let mut primary = MockVotingRule::new();
        let mut fallback = MockVotingRule::new();

        primary.expect_execute().times(1).return_const(Ok((
            RuleOutcome::MultipleWinners(vec![CandidateId::new(0, "A"), CandidateId::new(1, "B")]),
            Metrics::default(),
            Protocol::default(),
        )));

        fallback.expect_execute().times(1).return_const(Ok((
            RuleOutcome::UniqueWinner(CandidateId::new(1, "B")),
            Metrics::default(),
            Protocol::default(),
        )));

        let rule = Fallback::new(primary, fallback);

        let result = rule.execute(&fake_profile());

        assert_eq!(
            result.expect("The rule shouldn't fail").0,
            RuleOutcome::UniqueWinner(CandidateId::new(1, "B"))
        );
    }
    #[test]
    fn primary_error_is_propagated_and_fallback_not_called() {
        let mut primary = MockVotingRule::new();
        let mut fallback = MockVotingRule::new();

        primary
            .expect_execute()
            .times(1)
            .return_const(Err("primary failed"));

        fallback.expect_execute().times(0);

        let rule = Fallback::new(primary, fallback);

        let result = rule.execute(&fake_profile());

        assert!(matches!(
            result,
            Err(FallbackError::PrimaryError("primary failed"))
        ));
    }
    #[test]
    fn fallback_error_is_wrapped_correctly() {
        let mut primary = MockVotingRule::new();
        let mut fallback = MockVotingRule::new();

        primary.expect_execute().times(1).return_const(Ok((
            RuleOutcome::MultipleWinners(vec![CandidateId::new(0, "A"), CandidateId::new(1, "B")]),
            Metrics::default(),
            Protocol::default(),
        )));

        fallback
            .expect_execute()
            .times(1)
            .return_const(Err("fallback failed"));

        let rule = Fallback::new(primary, fallback);

        let result = rule.execute(&fake_profile());

        assert!(matches!(
            result,
            Err(FallbackError::FallbackError("fallback failed"))
        ));
    }
}
