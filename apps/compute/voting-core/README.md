# voting-core

`voting-core` is a **composable Rust framework for implementing voting rules**.

Instead of hard-coding monolithic algorithms, it models voting systems as
**pipelines of reusable steps**:

**scoring → decision → tie-breaking → elimination → fallback**

This design allows:
- implementing classic rules (Borda, Condorcet, Hare, Coombs, Copeland…)
- composing new rules without rewriting core logic
- experimenting with custom or hybrid voting systems

The crate is intended for **education, experimentation, and research**
in social choice theory.

---

## Example code snippets

TODO

---

## Core concepts

### Voting steps

Most ranked voting rules can be decomposed into three conceptual steps:

1. **Scoring**  
   Assigns numeric values to candidates (or candidate pairs).  
   (`Scorer` trait)

2. **Decision**  
   Selects a winner or a set of winners based on scores.  
   (`Decider` trait)

3. **Tie-breaking**  
   Resolves ties or leaves them unresolved, depending on the rule.  
   (`TieBreaker` trait)

These steps are designed to be **independent and composable**.

---

### Voting rule types

The crate provides two primary abstractions:

- **`VotingRule<S, D, T>`**  
  A single-stage pipeline composed of a scorer, decider, and tie-breaker.

- **`Elimination<S, E, D, T, Stop>`**  
  A multi-round elimination rule parameterized by:
  - a scorer
  - an elimination condition
  - a decider
  - a tie-breaker
  - a stopping condition

This supports elimination-based systems such as Hare or Coombs.

---

### Rule adaptors

Several higher-level adaptors are provided to modify rule behavior:

- **`Fallback<P, F>`**  
  Runs a primary rule `P`; if it yields multiple winners, runs fallback rule `F`.

- **`RequireUnique<R>`**  
  Ensures a rule returns a unique winner, otherwise returns an error.

- **`AcceptIf<R, F>`**  
  Accepts or rejects a rule’s result based on a predicate.

---

### Custom rules

For rules that do not fit the compositional model, the
**`VotingRuleExec`** trait can be implemented directly, allowing full
control over execution.

---

## Data model

### Profiles and candidates

A **`Profile`** represents a collection of ranked ballots.
It enforces several invariants at construction time:

- at least one voter
- at least one candidate
- all ballots have equal length
- all candidates are valid
- no duplicate candidates per ballot

**`CandidateId`** is a strongly-typed identifier used throughout the API.

---

### Pairwise data

- **`PairwiseMatrix`**  
  Encodes aggregated head-to-head preferences between candidates.

- **`CondorcetMatrix`**  
  A boolean transformation indicating pairwise victories, suitable for
  Condorcet-style methods.

---

## Project status and scope

This crate focuses on **correctness, composability, and clarity**.
UI layers, and election-specific constraints
are intentionally out of scope.

For internal design details and architectural rationale, see
[`ARCHITECTURE.md`](./ARCHITECTURE.md).
