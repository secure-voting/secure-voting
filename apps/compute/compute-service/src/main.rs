use std::{fmt::Debug, sync::OnceLock, time::Duration};

use opentelemetry::{KeyValue, global, trace::TracerProvider as _};
use opentelemetry_sdk::{
    Resource,
    metrics::{MeterProviderBuilder, PeriodicReader, SdkMeterProvider},
    trace::{RandomIdGenerator, Sampler, SdkTracerProvider},
};
use opentelemetry_semantic_conventions::{
    SCHEMA_URL,
    resource::{DEPLOYMENT_ENVIRONMENT_NAME, SERVICE_VERSION},
};
use tonic::Response;
use tracing::{Level, error, info, instrument};
use tracing_opentelemetry::{MetricsLayer, OpenTelemetryLayer};
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt};
use voting_core::prelude::*;

use crate::{
    metrics::ComputeMetrics,
    securevoting::compute::v1::{
        RunChunk, RunResult,
        ballot::Payload,
        compute_server::{Compute, ComputeServer},
        run_chunk::Part,
    },
};

#[allow(clippy::default_trait_access)]
#[allow(clippy::doc_markdown)]
#[allow(clippy::large_enum_variant)]
pub mod securevoting {
    pub mod compute {
        pub mod v1 {
            tonic::include_proto!("securevoting.compute.v1");
        }
    }
}

pub mod metrics;

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
struct ComputeService {
    metrics: ComputeMetrics,
}

#[tonic::async_trait]
impl Compute for ComputeService {
    #[instrument(skip(self))]
    async fn run(
        &self,
        request: tonic::Request<tonic::Streaming<RunChunk>>,
    ) -> Result<tonic::Response<RunResult>, tonic::Status> {
        println!("Stdout aboba");
        eprintln!("Stderr aboba");
        info!("Info aboba");
        error!("Error aboba");
        panic!("Panic aboba");
        std::process::exit(1);
        let start = std::time::Instant::now();
        self.metrics.active_requests.add(1, &[]);

        let _guard = scopeguard::guard((), |()| {
            self.metrics.active_requests.add(-1, &[]);
        });

        let (mut header, mut ballot_batch) = (None, vec![]);

        let (_metadatamap, _extensions, mut parts) = request.into_parts();

        while let Some(message_part) = parts.message().await? {
            let Some(message_part) = message_part.part else {
                continue;
            };

            match message_part {
                Part::Header(run_header) => {
                    if header.is_some() {
                        error!("Header was supplied twice!");
                        self.metrics.failures.add(1, &[]);
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
            error!("Ballot chunks are empty!");
            self.metrics.failures.add(1, &[]);
            return Ok(Response::new(create_error_type(
                tonic::Code::Internal,
                "empty ballot chunks",
            )));
        }

        let Some(header) = header else {
            error!("Header was not supplied!");
            self.metrics.failures.add(1, &[]);
            return Ok(Response::new(create_error_type(
                tonic::Code::Internal,
                "header was not supplied",
            )));
        };

        tracing::Span::current().record("tally_rule", &header.tally_rule);
        self.metrics
            .requests
            .add(1, &[KeyValue::new("rule", header.tally_rule.clone())]);

        let mut ballots = vec![];

        for batch in ballot_batch {
            for ballot in batch.ballots {
                let Some(ballot_payload) = ballot.payload else {
                    error!("Ballot payload was empty!");
                    self.metrics.failures.add(1, &[]);
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
                        error!("This ballot type is not yet supported!");
                        self.metrics.failures.add(1, &[]);
                        return Ok(Response::new(create_error_type(
                            tonic::Code::Unimplemented,
                            "not yet supported",
                        )));
                    }
                }
            }
        }

        if header.ballot_format != "ranking" {
            error!("This ballot type is not yet supported!");
            self.metrics.failures.add(1, &[]);
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
                error!("This calculation rule is not yet supported");
                self.metrics.failures.add(1, &[]);
                return Ok(Response::new(create_error_type(
                    tonic::Code::Unimplemented,
                    "not yet supported",
                )));
            }
        };
        match result {
            Ok(voting_results) => {
                info!("The results of the election are: {:?}", voting_results);
                self.metrics.duration.record(
                    start.elapsed().as_secs_f64(),
                    &[KeyValue::new("rule", header.tally_rule.clone())],
                );
                Ok(Response::new(create_winner_response(voting_results)))
            }
            Err(e) => {
                error!(
                    "The election process failed, error string: {}",
                    e.to_string()
                );
                self.metrics.failures.add(1, &[]);
                Ok(Response::new(create_error_type(
                    tonic::Code::InvalidArgument,
                    e.to_string(),
                )))
            }
        }
    }
}

fn resource() -> Resource {
    static RESOURCE: OnceLock<Resource> = OnceLock::new();

    RESOURCE
        .get_or_init(|| {
            Resource::builder()
                .with_service_name(env!("CARGO_PKG_NAME"))
                .with_schema_url(
                    [
                        KeyValue::new(SERVICE_VERSION, env!("CARGO_PKG_VERSION")),
                        KeyValue::new(DEPLOYMENT_ENVIRONMENT_NAME, "develop"),
                    ],
                    SCHEMA_URL,
                )
                .build()
        })
        .clone()
}

#[allow(clippy::expect_used)]
fn init_meter_provider() -> SdkMeterProvider {
    let exporter = opentelemetry_otlp::MetricExporter::builder()
        .with_tonic()
        .with_temporality(opentelemetry_sdk::metrics::Temporality::default())
        .build()
        .expect("Failed to create a MetricExporter");

    let reader = PeriodicReader::builder(exporter)
        .with_interval(Duration::from_secs(30))
        .build();

    let stdout_reader =
        PeriodicReader::builder(opentelemetry_stdout::MetricExporter::default()).build();

    let meter_provider = MeterProviderBuilder::default()
        .with_resource(resource())
        .with_reader(reader)
        .with_reader(stdout_reader)
        .build();

    global::set_meter_provider(meter_provider.clone());

    meter_provider
}

#[allow(clippy::expect_used)]
fn init_tracer_provider() -> SdkTracerProvider {
    let exporter = opentelemetry_otlp::SpanExporter::builder()
        .with_tonic()
        .build()
        .expect("Failed to create a SpanExporter");

    SdkTracerProvider::builder()
        .with_sampler(Sampler::ParentBased(Box::new(Sampler::TraceIdRatioBased(
            1.0,
        ))))
        .with_id_generator(RandomIdGenerator::default())
        .with_resource(resource())
        .with_batch_exporter(exporter)
        .build()
}

fn init_tracing_subscriber() -> OtelGuard {
    let tracer_provider = init_tracer_provider();
    let meter_provider = init_meter_provider();

    let tracer = tracer_provider.tracer("tracing-otel-subscriber");

    tracing_subscriber::registry()
        .with(tracing_subscriber::filter::LevelFilter::from_level(
            Level::INFO,
        ))
        .with(tracing_subscriber::fmt::layer())
        .with(MetricsLayer::new(meter_provider.clone()))
        .with(OpenTelemetryLayer::new(tracer))
        .init();

    OtelGuard {
        tracer_provider,
        meter_provider,
    }
}

struct OtelGuard {
    tracer_provider: SdkTracerProvider,
    meter_provider: SdkMeterProvider,
}

impl Drop for OtelGuard {
    fn drop(&mut self) {
        if let Err(err) = self.tracer_provider.shutdown() {
            eprintln!("{err:?}");
        }
        if let Err(err) = self.meter_provider.shutdown() {
            eprintln!("{err:?}");
        }
    }
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
    let _guard = init_tracing_subscriber();

    let addr: std::net::SocketAddr = std::env::var("GRPC_ADDR")
        .unwrap_or_else(|_| "0.0.0.0:50051".to_string())
        .parse()?;
    let service = ComputeService {
        metrics: ComputeMetrics::init(),
    };

    info!("Starting server on {}", addr);
    println!("BINARY VERSION MARKER 12345");

    tonic::transport::Server::builder()
        .layer(tonic_tracing_opentelemetry::middleware::server::OtelGrpcLayer::default())
        .add_service(ComputeServer::new(service))
        .serve(addr)
        .await?;

    Ok(())
}
