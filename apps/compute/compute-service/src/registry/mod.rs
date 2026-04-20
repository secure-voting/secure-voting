use std::collections::HashMap;

/// Implementations of the `Algrotihm` trait for the algorithms
/// provided by the voting-core library.
pub mod voting_rules;

/// An error type for the algorithm failure in calculations.
pub type MtError = String;

/// Error type for the algorithm execution pipeline in the registry.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum AlgorithmError {
    /// The provided ballot string cannot be converted into a ballot type.
    InvalidBallotType(String),
    /// The name of the algorithm doesn't correspond to anything registered.
    NoSuchAlgorithm,
    /// The provided algorithm doesn't support execution with this ballot type.
    UnsupportedBallotForAlgorithm {
        /// Algorithm's name.
        algorithm: String,
        /// Unsupported ballot type.
        ballot: BallotType,
    },
    /// Invalid argument for the algorithm itself.
    InvalidArgument(MtError),
}

/// Ballot type enum.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, PartialOrd, Ord)]
pub enum BallotType {
    /// Approval ballot type. Voters choose candidates to *approve*.
    Approval,
    /// Ranking ballot type. Voters *rank* their candidates based on preference.
    Ranking,
    /// Scoring ballot type. Voters score each candidate in a scale provided.
    Scoring,
}

impl TryFrom<&str> for BallotType {
    type Error = AlgorithmError;

    fn try_from(value: &str) -> Result<Self, Self::Error> {
        match value.to_lowercase().as_str() {
            "ranking" => Ok(BallotType::Ranking),
            "approval" => Ok(BallotType::Approval),
            "scoring" => Ok(BallotType::Scoring),
            x => Err(AlgorithmError::InvalidBallotType(x.to_owned())),
        }
    }
}

impl std::fmt::Display for BallotType {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        let s = match self {
            BallotType::Ranking => "ranking",
            BallotType::Approval => "approval",
            BallotType::Scoring => "scoring",
        };
        write!(f, "{s}")
    }
}

/// Unified algorithm trait.
///
/// Voting-core library exposes the VotingRuleExec type,
/// but it is not dyn-compatible.
///
/// This trait serves as a bridge between the computation
/// library and the registry.
pub trait Algorithm: std::fmt::Debug + Send + Sync {
    /// Run the election algorithm on the input data.
    ///
    /// # Errors
    ///
    /// The implementation of the algorithm is free to wrap its error type in the `AlgorithmError::InvalidArgument`.
    fn run_election(&self, input: Vec<Vec<String>>) -> Result<Vec<String>, AlgorithmError>;

    /// Alias of the algorithm. A short name for the system storage.
    fn alias(&self) -> &'static str;

    /// Whether the rule supports election calculations.
    fn supports_election_tally(&self) -> bool;

    /// Whether the rule supports experiment runs.
    fn supports_experiment_runs(&self) -> bool;

    /// Whether the rule requires the committee size to calculate.
    fn requires_committee_size(&self) -> bool;

    /// Whether the rule supports a quota type.
    fn supports_quota_type(&self) -> bool;

    /// Whether the approval maximal choices are bounded above.
    fn requires_approval_max_choices(&self) -> bool;

    /// Whether the rule supports only ranking top-k candidates instead of all of them.
    fn supports_ranking_top_k(&self) -> bool;

    /// Whether the rule requires a \[min, max\] constraint on the scores of candidates.
    fn requires_score_range(&self) -> bool;
}

/// Registry struct.
///
/// Registers algorithms, allows quering them, and executes them.
#[derive(Debug, Default)]
pub struct Registry {
    alias_map: HashMap<String, HashMap<BallotType, Box<dyn Algorithm>>>,
}

impl Registry {
    /// Construct an empty registry without algorithms.
    #[must_use]
    pub fn new() -> Self {
        Registry {
            alias_map: HashMap::new(),
        }
    }

    /// Add an algorithm with a ballot-type constraint to a registry.
    pub fn add(&mut self, algorithm: impl Algorithm + 'static, ballot_type: BallotType) -> bool {
        let alias = algorithm.alias().to_lowercase();

        let ballot_map = self.alias_map.entry(alias).or_default();

        if ballot_map.contains_key(&ballot_type) {
            return false;
        }

        ballot_map.insert(ballot_type, Box::new(algorithm));
        true
    }

    /// Execute a chosen algorithm on the input data and ballot type.
    ///
    /// # Errors
    ///
    /// There are several ways for this function to fail:
    ///
    /// 1. If the supplied ballot type string doesn't map to any valid ballot types
    /// 2. If no such algorithm is present by name in the registry
    /// 3. If the chosen algorithm doesn't support the ballot type chosen
    /// 4. If the algorithm itself returns the error while executing
    pub fn execute(
        &self,
        input: Vec<Vec<String>>,
        alias: &str,
        ballot_type: &str,
    ) -> Result<Vec<String>, AlgorithmError> {
        let ballot_type = BallotType::try_from(ballot_type)?;
        let alias_lower = alias.to_lowercase();

        let ballot_map = self
            .alias_map
            .get(&alias_lower)
            .ok_or(AlgorithmError::NoSuchAlgorithm)?;

        let algorithm =
            ballot_map
                .get(&ballot_type)
                .ok_or(AlgorithmError::UnsupportedBallotForAlgorithm {
                    algorithm: alias.to_string(),
                    ballot: ballot_type,
                })?;

        algorithm.run_election(input)
    }

    /// Get an iterator over the supported algorithms.
    pub fn algorithms(&self) -> impl Iterator<Item = &dyn Algorithm> {
        self.alias_map
            .values()
            .flat_map(|ballot_map| ballot_map.values())
            .map(AsRef::as_ref)
    }

    /// Return the supported ballot types.
    ///
    /// Chooses the ballot types that are supported
    /// by at least one algorithm in the registry.
    pub fn supported_ballots(&self, alias: &str) -> impl Iterator<Item = BallotType> + '_ {
        self.alias_map
            .get(&alias.to_lowercase())
            .into_iter()
            .flat_map(|m| m.keys().copied())
    }
}

#[cfg(test)]
#[allow(clippy::unwrap_used)]
mod tests {
    use super::*;
    use mockall::mock;

    mock! {
      #[derive(Debug)]
      pub Algorithm {

      }

      impl Algorithm for Algorithm {
        fn run_election(&self, input: Vec<Vec<String>>) -> Result<Vec<String>, AlgorithmError>;
        fn alias(&self) -> &'static str;
        fn supports_election_tally(&self) -> bool;
        fn supports_experiment_runs(&self) -> bool;
        fn requires_committee_size(&self) -> bool;
        fn supports_quota_type(&self) -> bool;
        fn requires_approval_max_choices(&self) -> bool;
        fn supports_ranking_top_k(&self) -> bool;
        fn requires_score_range(&self) -> bool;
      }
    }

    #[test]
    fn executes_registered_algorithm() {
        let mut registry = Registry::new();

        let mut algo = MockAlgorithm::new();
        algo.expect_alias().return_const_st("test");
        algo.expect_run_election()
            .return_const_st(Ok(vec!["A".into()]));

        registry.add(algo, BallotType::Ranking);

        let result = registry.execute(vec![vec!["A".into()]], "test", "ranking");

        assert!(result.is_ok());
    }

    #[test]
    fn unknown_algorithm_returns_error() {
        let registry = Registry::new();

        let err = registry
            .execute(vec![], "does_not_exist", "ranking")
            .unwrap_err();

        assert!(matches!(err, AlgorithmError::NoSuchAlgorithm));
    }

    #[test]
    fn unsupported_ballot_returns_error() {
        let mut registry = Registry::new();

        let mut algo = MockAlgorithm::new();
        algo.expect_alias().return_const_st("test");

        registry.add(algo, BallotType::Ranking);

        let err = registry.execute(vec![], "test", "approval").unwrap_err();

        assert!(matches!(
            err,
            AlgorithmError::UnsupportedBallotForAlgorithm { algorithm, ballot } if algorithm == "test" && ballot == BallotType::Approval
        ));
    }

    #[test]
    fn invalid_ballot_string_returns_error() {
        let registry = Registry::new();

        let err = registry
            .execute(vec![], "test", "not_a_ballot")
            .unwrap_err();

        assert!(matches!(err, AlgorithmError::InvalidBallotType(_)));
    }

    #[test]
    fn duplicate_registration_is_rejected() {
        let mut registry = Registry::new();

        let mut algo1 = MockAlgorithm::new();
        algo1.expect_alias().return_const_st("test");

        let mut algo2 = MockAlgorithm::new();
        algo2.expect_alias().return_const_st("test");

        let a = registry.add(algo1, BallotType::Ranking);
        let b = registry.add(algo2, BallotType::Ranking);

        assert!(a);
        assert!(!b);
    }

    #[test]
    fn supported_ballots_are_correct() {
        let mut registry = Registry::new();

        let mut algo1 = MockAlgorithm::new();
        algo1.expect_alias().return_const_st("test");

        let mut algo2 = MockAlgorithm::new();
        algo2.expect_alias().return_const_st("test");

        registry.add(algo1, BallotType::Approval);
        registry.add(algo2, BallotType::Ranking);

        let mut ballots: Vec<_> = registry.supported_ballots("test").collect();
        ballots.sort_by_key(|b| *b as u8);

        assert_eq!(ballots, vec![BallotType::Approval, BallotType::Ranking]);
    }

    #[test]
    fn unknown_alias_returns_empty_supported_ballots() {
        let registry = Registry::new();

        let ballots: Vec<_> = registry.supported_ballots("missing").collect();

        assert!(ballots.is_empty());
    }
}
