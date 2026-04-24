use voting_core::{
    models::approval::ApprovalBallot,
    prelude::*,
    tie_breaker::fallthrough::FallthroughTieBreaker,
    voting_rules::{Metrics, Protocol, approval::ApprovalRuleWith},
};

use crate::registry::{Algorithm, AlgorithmError, BallotType, Registry};

macro_rules! impl_algorithm {
    ($ty:path, $alias:expr, $tally:literal, $runs:literal, $size:literal, $type:literal, $choices:literal, $top_k:literal, $range: literal) => {
        impl Algorithm for $ty {
            fn run_election(
                &self,
                input: Vec<Vec<String>>,
            ) -> Result<(Vec<String>, Metrics, Protocol), AlgorithmError> {
                run_election(input, &Self::default())
                    .map_err(|e| AlgorithmError::InvalidArgument(e.to_string()))
            }

            fn alias(&self) -> &'static str {
                $alias
            }
            fn supports_election_tally(&self) -> bool {
                $tally
            }
            fn supports_experiment_runs(&self) -> bool {
                $runs
            }
            fn requires_committee_size(&self) -> bool {
                $size
            }
            fn supports_quota_type(&self) -> bool {
                $type
            }
            fn requires_approval_max_choices(&self) -> bool {
                $choices
            }
            fn supports_ranking_top_k(&self) -> bool {
                $top_k
            }
            fn requires_score_range(&self) -> bool {
                $range
            }
        }
    };
}

impl_algorithm!(
    BordaRule, "Borda", true, true, true, false, false, true, false
);
impl_algorithm!(
    PluralityRule,
    "Plurality",
    true,
    true,
    true,
    false,
    false,
    true,
    false
);
impl_algorithm!(
    ApprovalRule::<2>,
    "Approval-2",
    true,
    true,
    true,
    false,
    true,
    false,
    false
);
impl_algorithm!(
    ApprovalRule::<3>,
    "Approval-3",
    true,
    true,
    true,
    false,
    true,
    false,
    false
);
impl_algorithm!(
    ApprovalRuleWith::<2, FallthroughTieBreaker, ApprovalBallot>,
    "Approval-2",
    true,
    true,
    true,
    false,
    true,
    false,
    false
);
impl_algorithm!(
    ApprovalRuleWith::<3, FallthroughTieBreaker, ApprovalBallot>,
    "Approval-3",
    true,
    true,
    true,
    false,
    true,
    false,
    false
);
impl_algorithm!(
    AntiPluralityRule,
    "Inverse Plurality",
    true,
    true,
    true,
    false,
    false,
    true,
    false
);
impl_algorithm!(
    BlackRule, "Black", true, true, true, false, false, true, false
);
impl_algorithm!(
    CopelandIRule,
    "Copeland I",
    true,
    true,
    true,
    false,
    false,
    true,
    false
);
impl_algorithm!(
    CopelandIIRule,
    "Copeland II",
    true,
    true,
    true,
    false,
    false,
    true,
    false
);
impl_algorithm!(
    CopelandIIIRule,
    "Copeland III",
    true,
    true,
    true,
    false,
    false,
    true,
    false
);
impl_algorithm!(
    SimpsonRule,
    "Simpson",
    true,
    true,
    true,
    false,
    false,
    true,
    false
);
impl_algorithm!(
    MinmaxRule, "Minmax", true, true, true, false, false, true, false
);
impl_algorithm!(HareRule, "Hare", true, true, true, true, false, true, false);
impl_algorithm!(
    NansonRule, "Nanson", true, true, true, false, false, true, false
);
// impl_algorithm!(
//     CoombsRule, "Coombs", true, true, true, false, false, true, false
// );
impl_algorithm!(
    InverseBordaRule,
    "Inverse Borda",
    true,
    true,
    true,
    false,
    false,
    true,
    false
);
impl_algorithm!(
    QParetianStrongSimpleMajorityRule::<30>,
    "q-Paretian Strong Simple Majority",
    true,
    true,
    true,
    false,
    false,
    true,
    false
);
impl_algorithm!(
    QParetianStrongPluralityRule::<30>,
    "q-Paretian Strong Plurality",
    true,
    true,
    true,
    false,
    false,
    true,
    false
);
impl_algorithm!(
    QParetianStrongestSimpleMajorityRule::<30>,
    "q-Paretian Strongest Simple Majority",
    true,
    true,
    true,
    false,
    false,
    true,
    false
);

/// Return a registry with all the voting-core
/// algorithms included for appropriate ballot types.
#[must_use]
pub fn get_core_registry() -> Registry {
    let mut registry = Registry::new();
    registry.add(BordaRule::default(), BallotType::Ranking);
    registry.add(PluralityRule::default(), BallotType::Ranking);
    registry.add(ApprovalRule::<2>::default(), BallotType::Ranking);
    registry.add(ApprovalRule::<3>::default(), BallotType::Ranking);
    registry.add(AntiPluralityRule::default(), BallotType::Ranking);
    registry.add(BlackRule::default(), BallotType::Ranking);
    registry.add(CopelandIRule::default(), BallotType::Ranking);
    registry.add(CopelandIIRule::default(), BallotType::Ranking);
    registry.add(CopelandIIIRule::default(), BallotType::Ranking);
    registry.add(SimpsonRule::default(), BallotType::Ranking);
    registry.add(MinmaxRule::default(), BallotType::Ranking);
    registry.add(HareRule::default(), BallotType::Ranking);
    registry.add(NansonRule::default(), BallotType::Ranking);
    // registry.add(CoombsRule::default(), BallotType::Ranking);
    registry.add(InverseBordaRule::default(), BallotType::Ranking);
    registry.add(QParetianStrongSimpleMajorityRule, BallotType::Ranking);
    registry.add(QParetianStrongPluralityRule, BallotType::Ranking);
    registry.add(QParetianStrongestSimpleMajorityRule, BallotType::Ranking);

    registry.add(
        ApprovalRuleWith::<2, FallthroughTieBreaker, ApprovalBallot>::default(),
        BallotType::Approval,
    );
    registry.add(
        ApprovalRuleWith::<3, FallthroughTieBreaker, ApprovalBallot>::default(),
        BallotType::Approval,
    );

    registry
}
