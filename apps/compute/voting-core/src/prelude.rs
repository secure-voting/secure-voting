//! Prelude.
//!
//! Re-exports all the necessary public API:
//!
//! 1. 17 pre-packaged voting rules.
//! 2. Voting traits and adaptors.
//! 3. Matrix types, RuleOutcome, CandidateID and the Profile type.

// Voting rules:
// 1. Plurality
// 2. Approval voting q=2, 3
// 3. Inverse plurality
// 4. Borda
// 5. Black
// 6. Copeland I, II, III
// 7. Simpson (Maxmin)
// 8. Minmax
// 9. Strong q-Paretian simple majority
// 10. Strong q-Paretian plurality
// 11. Strongest q-Paretian simple majority
// 12. Condorcet practical
// 13. Threshold
// 14. Hare
// 15. Inverse Borda
// 16. Nanson
// 17. Coombs
pub use crate::voting_rules::{
    anti_plurality::AntiPluralityRule,
    approval::ApprovalRule,
    black::BlackRule,
    borda::BordaRule,
    copeland::{CopelandIIIRule, CopelandIIRule, CopelandIRule},
    plurality::PluralityRule,
};

// Voting traits.
pub use crate::{
    decider::Decider, scorer::Scorer, tie_breaker::TieBreaker, voting_rules::VotingRuleExec,
};

// Voting adaptors.
//
// Allow to combine the voting rules together or modify existing voting rules.
pub use crate::voting_rules::adaptors::*;

// Essential types.
pub use crate::{
    matrix::{CondorcetMatrix, PairwiseMatrix},
    profile::{CandidateId, Profile},
    tie_breaker::RuleOutcome,
};
