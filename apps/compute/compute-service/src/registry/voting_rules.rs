use voting_core::prelude::*;

use crate::registry::{Algorithm, AlgorithmError, Registry};

macro_rules! impl_algorithm {
    ($ty:path, $alias:expr) => {
        impl Algorithm for $ty {
            fn run_election(&self, input: Vec<Vec<String>>) -> Result<Vec<String>, AlgorithmError> {
                run_election(input, &Self::default())
                    .map_err(|e| AlgorithmError::InvalidArgument(e.to_string()))
            }

            fn alias(&self) -> &'static str {
                $alias
            }
        }
    };
}

impl_algorithm!(BordaRule, "borda");
impl_algorithm!(PluralityRule, "plurality");
impl_algorithm!(ApprovalRule::<2>, "approval-2");
impl_algorithm!(ApprovalRule::<3>, "approval-3");
impl_algorithm!(AntiPluralityRule, "inverse-pluarlity");
impl_algorithm!(BlackRule, "black");
impl_algorithm!(CopelandIRule, "copeland-i");
impl_algorithm!(CopelandIIRule, "copeland-ii");
impl_algorithm!(CopelandIIIRule, "copeland-iii");
impl_algorithm!(SimpsonRule, "simpson");
impl_algorithm!(MinmaxRule, "minmax");
impl_algorithm!(HareRule, "hare");
impl_algorithm!(NansonRule, "nanson");
impl_algorithm!(CoombsRule, "coombs");
impl_algorithm!(InverseBordaRule, "inverse-borda");

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
