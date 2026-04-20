use std::collections::HashMap;

pub mod voting_rules;

pub type MtError = String;
pub enum AlgorithmError {
    InvalidBallotType(String),
    NoSuchAlgorithm,
    UnsupportedBallotForAlgorithm {
        algorithm: String,
        ballot: BallotType,
    },
    InvalidArgument(MtError),
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum BallotType {
    Ranking,
    Approval,
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
        write!(f, "{}", s)
    }
}

pub trait Algorithm: std::fmt::Debug + Send + Sync {
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

#[derive(Debug, Default)]
pub struct Registry {
    alias_map: HashMap<String, HashMap<BallotType, Box<dyn Algorithm>>>,
}

impl Registry {
    pub fn new() -> Self {
        Registry {
            alias_map: HashMap::new(),
        }
    }

    pub fn add(&mut self, algorithm: impl Algorithm + 'static, ballot_type: BallotType) -> bool {
        let alias = algorithm.alias().to_lowercase();

        let ballot_map = self.alias_map.entry(alias).or_insert_with(HashMap::new);

        if ballot_map.contains_key(&ballot_type) {
            return false;
        }

        ballot_map.insert(ballot_type, Box::new(algorithm));
        true
    }

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

    pub fn algorithms(&self) -> impl Iterator<Item = &dyn Algorithm> {
        self.alias_map
            .values()
            .flat_map(|ballot_map| ballot_map.values())
            .map(|alg| alg.as_ref())
    }
    pub fn supported_ballots(&self, alias: &str) -> impl Iterator<Item = BallotType> + '_ {
        self.alias_map
            .get(&alias.to_lowercase())
            .into_iter()
            .flat_map(|m| m.keys().copied())
    }
}
