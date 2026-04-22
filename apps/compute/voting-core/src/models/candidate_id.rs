//! Module containing the `CandidateId` type implementatino

use std::fmt::Display;

/// Strongly-typed Candidate ID.
#[derive(Debug, PartialEq, Eq, Clone, Copy, Hash, PartialOrd, Ord)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct CandidateId(usize);

impl Display for CandidateId {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "candidate-id-{}", self.0)
    }
}

impl CandidateId {
    /// Create a new `CandidateId` instance.
    #[must_use]
    pub fn new(id: usize) -> Self {
        Self(id)
    }

    /// Get an inner numeric id.
    #[must_use]
    pub fn into_inner(self) -> usize {
        self.0
    }
}
