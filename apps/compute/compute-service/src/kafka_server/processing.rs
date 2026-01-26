use rdkafka::{Message, message::BorrowedMessage};
use voting_core::prelude::*;

use crate::kafka_server::models::ExperimentRunTask;

pub fn compute_voting_results(
    msg: BorrowedMessage,
) -> Result<RuleOutcome, Box<dyn std::error::Error + Send + Sync>> {
    let Some(payload) = msg.payload() else {
        return Err("payload is empty".to_owned().into());
    };

    let task_json: ExperimentRunTask = serde_json::from_slice(payload)?;
    let profile = create_profile_from_task_json(task_json)?;
}

fn create_profile_from_task_json(
    task_json: ExperimentRunTask,
) -> Result<Profile, Box<dyn std::error::Error + Send + Sync>> {
    todo!()
}
