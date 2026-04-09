use voting_core::prelude::*;

use crate::registry::{Algorithm, AlgorithmError, Registry};

macro_rules! impl_algorithm {
    ($ty:path, $alias:expr, $ballots: expr, $tally:literal, $runs:literal, $size:literal, $type:literal, $choices:literal, $top_k:literal, $range: literal) => {
        impl Algorithm for $ty {
            fn run_election(&self, input: Vec<Vec<String>>) -> Result<Vec<String>, AlgorithmError> {
                run_election(input, &Self::default())
                    .map_err(|e| AlgorithmError::InvalidArgument(e.to_string()))
            }

            fn alias(&self) -> &'static str {
                $alias
            }
            fn ballot_formats(&self) -> &[&'static str] {
                $ballots
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
    BordaRule,
    "borda",
    &["ranking"],
    true,
    true,
    true,
    false,
    false,
    true,
    false
);
impl_algorithm!(
    PluralityRule,
    "plurality",
    &["ranking"],
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
    "approval-2",
    &["ranking"],
    false,
    true,
    true,
    false,
    true,
    false,
    false
);
impl_algorithm!(
    ApprovalRule::<3>,
    "approval-3",
    &["ranking"],
    false,
    true,
    true,
    false,
    true,
    false,
    false
);
impl_algorithm!(
    AntiPluralityRule,
    "inverse-pluarlity",
    &["ranking"],
    true,
    true,
    true,
    false,
    false,
    true,
    false
);
impl_algorithm!(
    BlackRule,
    "black",
    &["ranking"],
    true,
    true,
    true,
    false,
    false,
    true,
    false
);
impl_algorithm!(
    CopelandIRule,
    "copeland-i",
    &["ranking"],
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
    "copeland-ii",
    &["ranking"],
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
    "copeland-iii",
    &["ranking"],
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
    "simpson",
    &["ranking"],
    true,
    true,
    true,
    false,
    false,
    true,
    false
);
impl_algorithm!(
    MinmaxRule,
    "minmax",
    &["ranking"],
    true,
    true,
    true,
    false,
    false,
    true,
    false
);
impl_algorithm!(
    HareRule,
    "hare",
    &["ranking"],
    true,
    true,
    true,
    true,
    false,
    true,
    false
);
impl_algorithm!(
    NansonRule,
    "nanson",
    &["ranking"],
    true,
    true,
    true,
    false,
    false,
    true,
    false
);
impl_algorithm!(
    CoombsRule,
    "coombs",
    &["ranking"],
    true,
    true,
    true,
    false,
    false,
    true,
    false
);
impl_algorithm!(
    InverseBordaRule,
    "inverse-borda",
    &["ranking"],
    true,
    true,
    true,
    false,
    false,
    true,
    false
);

pub fn get_core_registry() -> Registry {
    let mut registry = Registry::new();
    registry.add(BordaRule::default());
    registry.add(PluralityRule::default());
    registry.add(ApprovalRule::<2>::default());
    registry.add(ApprovalRule::<3>::default());
    registry.add(AntiPluralityRule::default());
    registry.add(BlackRule::default());
    registry.add(CopelandIRule::default());
    registry.add(CopelandIIRule::default());
    registry.add(CopelandIIIRule::default());
    registry.add(SimpsonRule::default());
    registry.add(MinmaxRule::default());
    registry.add(HareRule::default());
    registry.add(NansonRule::default());
    registry.add(CoombsRule::default());
    registry.add(InverseBordaRule::default());

    registry
}
