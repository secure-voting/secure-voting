use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::collections::HashMap;

#[derive(Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub struct ExperimentRunTask {
    pub kind: String,
    pub job_id: String,
    pub run_id: String,
    pub experiment_id: String,
    pub dataset_id: String,

    pub experiment_type: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub experiment_seed: Option<i64>,
    pub experiment_params: Value,

    pub dataset: DatasetInfo,
}

#[derive(Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub struct DatasetInfo {
    pub id: String,
    pub name: String,
    pub description: Option<String>,
    pub source: String,
    pub format: String,
    pub candidates: Vec<DatasetCandidate>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub seed: Option<i64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub parameters: Option<HashMap<String, Value>>,
    pub created_at: String,
}

#[derive(Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub struct DatasetCandidate {
    pub id: String,
    pub name: String,
}

#[derive(Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub struct ExperimentRunResult {
    pub kind: String,
    pub run_id: String,
    pub status: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error_text: Option<String>,

    #[serde(skip_serializing_if = "Option::is_none")]
    pub winners: Option<Vec<Value>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub metrics: Option<HashMap<String, Value>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub timings: Option<HashMap<String, Value>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub artifacts: Option<HashMap<String, Value>>,
}
