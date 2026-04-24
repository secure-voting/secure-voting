//! Main computing service.
//!
//! Responds to requests for calulcation and the available algorithm list.
//! Uses the voting-core library to calculate election results.

#![warn(missing_docs)]
#![warn(clippy::missing_docs_in_private_items)]
#![forbid(unsafe_code)]

use std::sync::{Arc, RwLock};

use tonic::{
    Response,
    transport::{Identity, server::ServerTlsConfig},
};
use voting_core::voting_rules::{Metrics, Protocol};

use crate::{
    registry::{AlgorithmError, Registry, voting_rules::get_core_registry},
    securevoting::compute::v1::{
        ListTallyRulesResponse, RunChunk, RunResult, TallyRuleInfo,
        ballot::Payload,
        compute_server::{Compute, ComputeServer},
        run_chunk::Part,
    },
};

#[allow(clippy::default_trait_access)]
#[allow(clippy::doc_markdown)]
#[allow(clippy::large_enum_variant)]
#[allow(clippy::struct_excessive_bools)]
#[allow(clippy::too_many_lines)]
/// Generated proto-structs.
pub mod securevoting {
    #[allow(missing_docs)]
    pub mod compute {
        pub mod v1 {
            tonic::include_proto!("securevoting.compute.v1");
        }
    }
}

/// Registry module.
///
/// Contains the implementaions of the `Algorithm` trait and the `Registry` structure.
pub mod registry;

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

#[allow(clippy::expect_used)]
fn create_winner_response(winners: Vec<String>, metrics: Metrics, protocol: Protocol) -> RunResult {
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
        metrics_json: serde_json::to_vec(&metrics).expect("Serialization failed"),
        protocol_json: serde_json::to_vec(&protocol).expect("Serialization failed"),
        timings_json: vec![],
        artifacts_json: vec![],
    }
}

#[derive(Debug, Default)]
struct ComputeService {
    registry: Arc<RwLock<Registry>>,
}

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
                    Payload::Approval(approval_ballot) => {
                        ballots.push(approval_ballot.approvals);
                    }
                    Payload::Score(_) => {
                        return Ok(Response::new(create_error_type(
                            tonic::Code::Unimplemented,
                            "not yet supported",
                        )));
                    }
                }
            }
        }

        if header.ballot_format != "ranking" && header.ballot_format != "approval" {
            return Ok(Response::new(create_error_type(
                tonic::Code::Unimplemented,
                "not yet supported",
            )));
        }

        #[allow(clippy::expect_used)]
        match self.registry.read().expect("RwLock is poisoned").execute(
            ballots,
            header.tally_rule.as_str(),
            &header.ballot_format,
        ) {
            Ok(result) => Ok(Response::new(create_winner_response(
                result.0, result.1, result.2,
            ))),
            Err(AlgorithmError::NoSuchAlgorithm(a)) => Ok(Response::new(create_error_type(
                tonic::Code::Unimplemented,
                format!("No such algorithm: {a}"),
            ))),
            Err(AlgorithmError::InvalidArgument(e) | AlgorithmError::InvalidBallotType(e)) => Ok(
                Response::new(create_error_type(tonic::Code::InvalidArgument, e)),
            ),
            Err(AlgorithmError::UnsupportedBallotForAlgorithm { algorithm, ballot }) => {
                Ok(Response::new(create_error_type(
                    tonic::Code::InvalidArgument,
                    format!("Algorithm {algorithm} does not support ballot type {ballot}"),
                )))
            }
        }
    }

    #[allow(clippy::expect_used)]
    async fn list_tally_rules(
        &self,
        _request: tonic::Request<()>,
    ) -> Result<tonic::Response<ListTallyRulesResponse>, tonic::Status> {
        Ok(tonic::Response::new(ListTallyRulesResponse {
            rules: self
                .registry
                .read()
                .expect("RwLock is poisoned")
                .algorithms()
                .map(|algo| TallyRuleInfo {
                    id: algo.alias().to_lowercase().replace(' ', "-"),
                    label: algo.alias().to_owned(),
                    ballot_formats: self
                        .registry
                        .read()
                        .expect("RwLock is poisoned")
                        .supported_ballots(algo.alias())
                        .map(|x| x.to_string())
                        .collect(),
                    supports_election_tally: algo.supports_election_tally(),
                    supports_experiment_runs: algo.supports_experiment_runs(),
                    requires_committee_size: algo.requires_committee_size(),
                    supports_quota_type: algo.supports_quota_type(),
                    requires_approval_max_choices: algo.requires_approval_max_choices(),
                    supports_ranking_top_k: algo.supports_ranking_top_k(),
                    requires_score_range: algo.requires_score_range(),
                })
                .collect(),
        }))
    }
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let registry = get_core_registry();

    let addr: std::net::SocketAddr = std::env::var("GRPC_ADDR")
        .unwrap_or_else(|_| "0.0.0.0:50051".to_string())
        .parse()?;
    let server = ComputeService {
        registry: Arc::new(RwLock::new(registry)),
    };

    let cert = std::fs::read_to_string(format!(
        "{}/../../../scripts/certs/out/compute.pem",
        env!("CARGO_MANIFEST_DIR")
    ))?;
    let key = std::fs::read_to_string(format!(
        "{}/../../../scripts/certs/out/compute.key",
        env!("CARGO_MANIFEST_DIR")
    ))?;

    let tls_config = ServerTlsConfig::new().identity(Identity::from_pem(&cert, &key));

    tonic::transport::Server::builder()
        .tls_config(tls_config)?
        .concurrency_limit_per_connection(256)
        .add_service(ComputeServer::new(server))
        .serve(addr)
        .await?;

    Ok(())
}
