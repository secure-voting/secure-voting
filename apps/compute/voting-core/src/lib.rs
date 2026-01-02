//! voting-core is the core computation library for the Rust Voting Compute Service
//!
//! It computes the result of the vote according to the chosen VotingRule strategy.
//! VotingRule is modular, meaning you can choose a Scorer, a Decider and a TieBreaker as long as
//! they have matching in/out types (the compiler will let you know).

#![warn(missing_docs)]
#![warn(clippy::missing_docs_in_private_items)]

pub mod decider;
pub mod profile;
pub mod scorer;
pub mod tie_breaker;
pub mod voting_rules;
