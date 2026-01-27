use tonic::Response;
use voting_core::prelude::*;

use crate::securevoting::compute::v1::{
    RunChunk, RunResult,
    ballot::Payload,
    compute_server::{Compute, ComputeServer},
    run_chunk::Part,
};

#[allow(clippy::all)]
pub mod securevoting {
    pub mod compute {
        pub mod v1 {
            tonic::include_proto!("securevoting.compute.v1");
        }
    }
}

fn create_error_type(code: tonic::Code, message: impl Into<String>) -> RunResult {
    RunResult {
        method: String::new(),
        params_json: vec![],

        status: "error".to_owned(),
        error_text: format!("ErrorCode: {code}, details: {}", message.into()),
        winners_json: vec![],
        metrics_json: vec![],
        protocol_json: vec![],
        timings_json: vec![],
        artifacts_json: vec![],
    }
}

fn create_winner_response(winners: Vec<String>) -> RunResult {
    let winner_json = format!(
        "[{}]",
        winners
            .into_iter()
            .map(|x| format!("\"{x}\""))
            .collect::<Vec<_>>()
            .join(", ")
    );

    RunResult {
        method: String::new(),
        params_json: vec![],
        
        status: "done".to_owned(),
        error_text: String::new(),
        winners_json: winner_json.as_bytes().to_vec(),
        metrics_json: vec![],
        protocol_json: vec![],
        timings_json: vec![],
        artifacts_json: vec![],
    }
}

#[derive(Debug, Default)]
struct ComputeService;

#[tonic::async_trait]
impl Compute for ComputeService {
    async fn run(
        &self,
        request: tonic::Request<tonic::Streaming<RunChunk>>,
    ) -> Result<tonic::Response<RunResult>, tonic::Status> {
        let (mut header, mut ballot_batch) = (None, vec![]);

        let (_metadatamap, _extensions, mut parts) = request.into_parts();

        while let Some(message_part) = parts.message().await? {
            let Some(message_part) = message_part.part else {
                continue;
            };

            match message_part {
                Part::Header(run_header) => {
                    if header.is_some() {
                        return Ok(Response::new(create_error_type(
                            tonic::Code::Internal,
                            "header was supplied twice",
                        )));
                    }

                    header = Some(run_header);
                }
                Part::Batch(b_batch) => {
                    ballot_batch.push(b_batch);
                }
            }
        }

        if ballot_batch.is_empty() {
            return Ok(Response::new(create_error_type(
                tonic::Code::Internal,
                "empty ballot chunks",
            )));
        }

        let Some(header) = header else {
            return Ok(Response::new(create_error_type(
                tonic::Code::Internal,
                "header was not supplied",
            )));
        };

        let mut ballots = vec![];

        for batch in ballot_batch {
            for ballot in batch.ballots {
                let Some(ballot_payload) = ballot.payload else {
                    return Ok(Response::new(create_error_type(
                        tonic::Code::InvalidArgument,
                        "empty ballot paylod",
                    )));
                };

                match ballot_payload {
                    Payload::Ranking(ranking_ballot) => {
                        ballots.push(ranking_ballot.ranking);
                    }
                    _ => {
                        return Ok(Response::new(create_error_type(
                            tonic::Code::Unimplemented,
                            "not yet supported",
                        )));
                    }
                }
            }
        }

        if header.ballot_format != "ranking" {
            return Ok(Response::new(create_error_type(
                tonic::Code::Unimplemented,
                "not yet supported",
            )));
        }

        let result = match header.tally_rule.as_str() {
            "borda" => run_election(ballots, &BordaRule::default()),
            "plurality" => run_election(ballots, &PluralityRule::default()),
            "approval-2" => run_election(ballots, &ApprovalRule::<2>::default()),
            "approval-3" => run_election(ballots, &ApprovalRule::<3>::default()),
            "inverse-plurality" => run_election(ballots, &AntiPluralityRule::default()),
            "black" => run_election(ballots, &BlackRule::default()),
            "copeland-i" => run_election(ballots, &CopelandIRule::default()),
            "copeland-ii" => run_election(ballots, &CopelandIIRule::default()),
            "copeland-iii" => run_election(ballots, &CopelandIIIRule::default()),
            "simpson" => run_election(ballots, &SimpsonRule::default()),
            "Minmax" => run_election(ballots, &MinmaxRule::default()),
            "hare" => run_election(ballots, &HareRule::default()),
            "nanson" => run_election(ballots, &NansonRule::default()),
            "coombs" => run_election(ballots, &CoombsRule::default()),
            "inverse-borda" => run_election(ballots, &InverseBordaRule::default()),
            _ => {
                return Ok(Response::new(create_error_type(
                    tonic::Code::Unimplemented,
                    "not yet supported",
                )));
            }
        };
        match result {
            Ok(voting_results) => Ok(Response::new(create_winner_response(voting_results))),
            Err(e) => Ok(Response::new(create_error_type(
                tonic::Code::InvalidArgument,
                e.to_string(),
            ))),
        }
    }
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let addr: std::net::SocketAddr = std::env::var("GRPC_ADDR")
    .unwrap_or_else(|_| "0.0.0.0:50051".to_string())
    .parse()?;
    let greeter = ComputeService;

    tonic::transport::Server::builder()
        .add_service(ComputeServer::new(greeter))
        .serve(addr)
        .await?;

    Ok(())
}
