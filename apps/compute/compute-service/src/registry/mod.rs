use std::collections::HashMap;

pub mod voting_rules;

pub type MtError = String;
pub enum AlgorithmError {
    NoSuchAlgorithm,
    InvalidArgument(MtError),
}

pub trait Algorithm: std::fmt::Debug + Send + Sync {
    fn run_election(&self, input: Vec<Vec<String>>) -> Result<Vec<String>, AlgorithmError>;
    fn alias(&self) -> &'static str;
    fn ballot_formats(&self) -> &[&'static str];
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
    algorithms: Vec<Box<dyn Algorithm>>,
    alias_map: HashMap<String, usize>,
}

impl Registry {
    pub fn new() -> Self {
        Registry {
            algorithms: vec![],
            alias_map: HashMap::new(),
        }
    }

    pub fn add(&mut self, algorithm: impl Algorithm + 'static) -> bool {
        if self
            .alias_map
            .contains_key(&algorithm.alias().to_lowercase())
        {
            return false;
        }

        self.alias_map
            .insert(algorithm.alias().to_lowercase(), self.algorithms.len());
        self.algorithms.push(Box::new(algorithm));
        true
    }

    pub fn execute(
        &self,
        input: Vec<Vec<String>>,
        alias: &str,
    ) -> Result<Vec<String>, AlgorithmError> {
        let index = *self
            .alias_map
            .get(&alias.to_lowercase())
            .ok_or(AlgorithmError::NoSuchAlgorithm)?;
        self.algorithms[index].run_election(input)
    }
}
