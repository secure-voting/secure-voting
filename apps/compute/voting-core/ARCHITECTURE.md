# Architecture

This document describes the internal structure, invariants, and design
decisions of the `voting-core` crate. It is intended for contributors and
maintainers.

---

## Design philosophy

The crate follows three guiding principles:

1. **Decomposition**  
   Voting rules are decomposed into minimal, reusable steps.

2. **Static correctness**  
   Invariants are enforced through types wherever possible.

3. **Extensibility over completeness**  
   The framework prioritizes enabling new rules over exhaustively
   implementing all known systems.

---

## Internal abstractions

### Voting steps

Each voting step is represented by a trait:

- `Scorer`
- `Decider`
- `TieBreaker`

These traits are intentionally narrow to allow independent reuse and
testing. Most rule implementations are simple compositions of these
components.

---

### Elimination model

Elimination-based rules are modeled as repeated applications of:

- a scorer
- an elimination criterion
- a decision step
- an optional tie-breaker
- a stopping condition

This allows expressing rules such as Hare and Coombs without duplicating
control flow logic.

---

### Adaptors

Adaptors wrap existing voting rules and alter their behavior without
modifying the underlying logic. Examples include enforcing uniqueness
or providing fallback strategies.

Adaptors are preferred over branching logic inside rule implementations.

---

## Data invariants

### Profile

The `Profile` type enforces the following invariants:

1. At least one voter.
2. At least one candidate.
3. Uniform ballot length.
4. Valid candidate references.
5. No duplicate candidates per ballot.

This significantly reduces validation logic in downstream algorithms.

---

### Pairwise matrices

`PairwiseMatrix` guarantees:

- non-empty square shape
- zero diagonal
- symmetry:  
  `matrix[i][j] + matrix[j][i] = n` for `i ≠ j`

`CondorcetMatrix` is derived from `PairwiseMatrix` and encodes pairwise
victories using boolean values.

---

## Module structure

```
src/
├── decider # Decider trait
├── scorer # Scorer trait
├── tie_breaker # TieBreaker trait
├── matrix # Pairwise and Condorcet matrices
├── profile.rs # Profile and CandidateId
├── voting_rules # Rule composition and implementations
│ ├── adaptors
│ └── elimination
│     ├── criterion
│     └── stop
├── prelude.rs # Public re-exports
└── lib.rs # Crate entry point
```

Module internals are intentionally kept private where possible to avoid
leaking implementation details into the public API.

---

## Extensibility notes

If a voting rule cannot be naturally expressed through the existing
pipeline model, implementing `VotingRuleExec` is the intended escape
hatch. This is considered a feature, not a failure of the abstraction.
