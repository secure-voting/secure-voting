//! Prelude with re-exports of all the necessary public API.
//!
//! 1. Pre-packaged common voting rules.
//! 2. Voting traits and adaptors.
//! 3. Matrix types, `RuleOutcome`, `CandidateID` and the Profile type.

// Voting rules:
// 1. Plurality
// 2. Approval voting q=2, 3
// 3. Inverse plurality
// 4. Borda
// 5. Black
// 6. Copeland I, II, III
// 7. Simpson (Maxmin)
// 8. Minmax
// 9. Condorcet practical
// 10. Hare
// 11. Nanson
// 12. Coombs
// 13. Inverse Borda
//
// TBD:
//
// 1. Strong q-Paretian simple majority
// 2. Strong q-Paretian plurality
// 3. Strongest q-Paretian simple majority
pub use crate::voting_rules::{
    anti_plurality::AntiPluralityRule,
    approval::ApprovalRule,
    black::BlackRule,
    borda::BordaRule,
    coombs::CoombsRule,
    copeland::{CopelandIIIRule, CopelandIIRule, CopelandIRule},
    hare::HareRule,
    inverse_borda::InverseBordaRule,
    minmax::MinmaxRule,
    nanson::NansonRule,
    plurality::PluralityRule,
    practical_condorcet::CondorcetPracticalRule,
    simpson::SimpsonRule,
};

// Voting traits.
pub use crate::{
    decider::Decider,
    scorer::Scorer,
    tie_breaker::TieBreaker,
    voting_rules::{
        VotingRuleExec,
        elimination::{criterion::EliminationCriterion, stop::EliminationStopCondition},
    },
};

// Voting adaptors.
//
// Allow to combine the voting rules together or modify existing voting rules.
pub use crate::voting_rules::adaptors::*;

// Essential types and functions.
pub use crate::{
    election::run_election,
    matrix::{CondorcetMatrix, PairwiseMatrix},
    models::{candidate_id::CandidateId, profile::Profile},
    tie_breaker::RuleOutcome,
    voting_rules::{
        elimination::rule::{Elimination, EliminationRuleError},
        voting_rule::{VotingRule, VotingRuleError},
    },
};
