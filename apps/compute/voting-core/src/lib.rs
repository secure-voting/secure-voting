//! voting-core is the core computation library for the Rust Voting Compute Service.
//!
//! It computes election outcomes using composable voting primitives rather than
//! monolithic algorithms.
//!
//! The crate provides two primary execution models:
//!
//! 1. `VotingRule` — a single-round election composed of a `Scorer`, `Decider`,
//!    and `TieBreaker`.
//!
//! 2. `Elimination` — a multi-round elimination election (e.g. IRV, Coombs,
//!    Nanson), composed of a `Scorer`, `Decider`, `TieBreaker`,
//!    `EliminationCriterion`, and `EliminationStopCondition`.

#![warn(missing_docs)]
#![warn(clippy::missing_docs_in_private_items)]
#![forbid(unsafe_code)]

pub mod decider;
pub mod matrix;
pub mod prelude;
pub mod profile;
pub mod scorer;
pub mod tie_breaker;
pub mod voting_rules;
