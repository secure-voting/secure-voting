use std::{sync::OnceLock, time::Duration};

use opentelemetry::{KeyValue, global};
use opentelemetry_appender_tracing::layer;
use opentelemetry_otlp::{MetricExporter, Protocol, SpanExporter, WithExportConfig};
use opentelemetry_sdk::{
    Resource, logs::SdkLoggerProvider, metrics::SdkMeterProvider, trace::SdkTracerProvider,
};
use tonic::Response;
use tower_http::trace::TraceLayer;
use tracing::{error, info, instrument};
use tracing_subscriber::{EnvFilter, prelude::*};
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
    #[instrument(skip(self, request), fields(tally_rule = tracing::field::Empty))]
    async fn run(
        &self,
        request: tonic::Request<tonic::Streaming<RunChunk>>,
    ) -> Result<tonic::Response<RunResult>, tonic::Status> {
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

fn get_resource() -> Resource {
    static RESOURCE: OnceLock<Resource> = OnceLock::new();
    RESOURCE
        .get_or_init(|| {
            Resource::builder()
                .with_service_name("rust-compute-service")
                .build()
        })
        .clone()
}

#[allow(clippy::expect_used)]
#[allow(clippy::unwrap_used)]
fn init_logs() -> SdkLoggerProvider {
    let exporter = opentelemetry_otlp::LogExporter::builder()
        .with_tonic()
        .with_endpoint(
            std::env::var("OTEL_COLLECTOR_URL").unwrap_or("http://otel-collector:4317".to_owned()),
        )
        .with_protocol(Protocol::Grpc)
        .with_timeout(Duration::from_secs(10))
        .build()
        .expect("Failed to create a log exporter");
    let logger_provider = SdkLoggerProvider::builder()
        .with_batch_exporter(exporter)
        .with_resource(get_resource())
        .build();

    let filter_otel = EnvFilter::new("info")
        .add_directive("tonic=off".parse().unwrap())
        .add_directive("hyper=off".parse().unwrap())
        .add_directive("h2=off".parse().unwrap())
        .add_directive("reqwest=off".parse().unwrap());
    let otel_layer =
        layer::OpenTelemetryTracingBridge::new(&logger_provider).with_filter(filter_otel);

    let filter_fmt = EnvFilter::new("info");
    let fmt_layer = tracing_subscriber::fmt::layer()
        .with_thread_names(true)
        .with_filter(filter_fmt);

    tracing_subscriber::registry()
        .with(otel_layer)
        .with(fmt_layer)
        .init();

    logger_provider
}

#[allow(clippy::expect_used)]
fn init_traces() -> SdkTracerProvider {
    let exporter = SpanExporter::builder()
        .with_tonic()
        .with_endpoint(
            std::env::var("OTEL_COLLECTOR_URL").unwrap_or("http://otel-collector:4317".to_owned()),
        )
        .with_protocol(Protocol::Grpc)
        .with_timeout(Duration::from_secs(10))
        .build()
        .expect("Failed to create a trace exporter");

    SdkTracerProvider::builder()
        .with_batch_exporter(exporter)
        .with_resource(get_resource())
        .build()
}

#[allow(clippy::expect_used)]
fn init_metrics() -> SdkMeterProvider {
    let exporter = MetricExporter::builder()
        .with_tonic()
        .with_endpoint(
            std::env::var("OTEL_COLLECTOR_URL").unwrap_or("http://otel-collector:4317".to_owned()),
        )
        .with_protocol(Protocol::Grpc)
        .with_timeout(Duration::from_secs(10))
        .build()
        .expect("Failed to create a metric exporter");

    SdkMeterProvider::builder()
        .with_periodic_exporter(exporter)
        .with_resource(get_resource())
        .build()
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
    let logger_provider = init_logs();

    let tracer_provider = init_traces();
    global::set_tracer_provider(tracer_provider.clone());

    let meter_provider = init_metrics();
    global::set_meter_provider(meter_provider.clone());

    let addr: std::net::SocketAddr = std::env::var("GRPC_ADDR")
        .unwrap_or_else(|_| "0.0.0.0:50051".to_string())
        .parse()?;
    let service = ComputeService {
        metrics: ComputeMetrics::init(),
    };

    info!("Starting server on {}", addr);

    tonic::transport::Server::builder()
        .layer(TraceLayer::new_for_grpc())
        .add_service(ComputeServer::new(service))
        .serve(addr)
        .await?;

    let mut shutdown_errors = Vec::new();
    if let Err(e) = logger_provider.shutdown() {
        shutdown_errors.push(format!("logger provider: {e}"));
    }
    if let Err(e) = tracer_provider.shutdown() {
        shutdown_errors.push(format!("tracer provider: {e}"));
    }

    if let Err(e) = meter_provider.shutdown() {
        shutdown_errors.push(format!("meter provider: {e}"));
    }

    if !shutdown_errors.is_empty() {
        return Err(format!(
            "Failed to shutdown providers: {}",
            shutdown_errors.join("\n")
        )
        .into());
    }

    Ok(())
}
