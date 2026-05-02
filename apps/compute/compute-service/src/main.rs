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

use compute_service::registry::Registry;
use compute_service::registry::voting_rules::get_core_registry;
use compute_service::securevoting::compute::v1::{
    ListTallyRulesResponse, RunChunk, RunResult, TallyRuleInfo,
    compute_server::{Compute, ComputeServer},
    run_chunk::Part,
};
use compute_service::{create_error_type, process_request};

/// Compute service struct.
#[derive(Debug, Default)]
struct ComputeService {
    /// Algorithmic registry.
    registry: Arc<RwLock<Registry>>,
}

#[allow(clippy::cast_sign_loss)]
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

        #[allow(clippy::expect_used)]
        let registry = self.registry.read().expect("RwLock is poisoned");
        Ok(Response::new(process_request(
            &header,
            &ballot_batch,
            &registry,
        )))
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
