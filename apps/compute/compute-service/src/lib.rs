//! Compute service library.
//!
//! Provides the gRPC service implementation for running voting elections.

#![warn(missing_docs)]
#![warn(clippy::missing_docs_in_private_items)]
#![forbid(unsafe_code)]

use std::collections::HashMap;

use voting_core::models::{BallotData, candidate_id::CandidateId};
use voting_core::voting_rules::{Metrics, Protocol};

pub use registry::{AlgorithmError, Registry, voting_rules::get_core_registry};
pub use securevoting::compute::v1::{BallotBatch, RunHeader, RunResult};
pub use system_metrics::SystemMetricsCollector;

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
pub mod registry;
/// System metrics collection.
pub mod system_metrics;

use crate::securevoting::compute::v1::ballot::Payload;

/// Helper function to create a response containing an error.
pub fn create_error_type(code: tonic::Code, message: impl Into<String>) -> RunResult {
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

/// Helper function to create a response containing the winner,
/// metrics, protocol steps and timings.
///
/// # Panics
///
/// Panics if the serialization of the metrics or the protocol fails.
/// Shouldn't happen, as both implement `Serialize`.
#[allow(clippy::expect_used)]
#[must_use]
pub fn create_winner_response(
    winners: Vec<String>,
    metrics: &Metrics,
    protocol: &Protocol,
    timings_json: &[u8],
) -> RunResult {
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
        metrics_json: serde_json::to_vec(metrics).expect("Serialization failed"),
        protocol_json: serde_json::to_vec(protocol).expect("Serialization failed"),
        timings_json: timings_json.to_vec(),
        artifacts_json: vec![],
    }
}

/// Parse the ballot stream into `BallotData`.
///
/// # Errors
///
/// Returns an error if the payload of the ballot is empty.
#[allow(clippy::cast_sign_loss)]
#[allow(clippy::implicit_hasher)]
pub fn parse_ballots(
    batches: &[BallotBatch],
    candidate_map: &HashMap<&str, usize>,
) -> Result<Vec<BallotData>, String> {
    let mut ballots = vec![];

    for batch in batches {
        for ballot in &batch.ballots {
            let Some(ballot_payload) = ballot.payload.clone() else {
                return Err("empty ballot paylod".into());
            };

            match ballot_payload {
                Payload::Ranking(ranking_ballot) => {
                    let ranked: Vec<CandidateId> = ranking_ballot
                        .ranking
                        .iter()
                        .filter_map(|name| {
                            candidate_map
                                .get(name.as_str())
                                .map(|&idx| CandidateId::new(idx, name.clone()))
                        })
                        .collect();
                    ballots.push(BallotData::Simple(ranked));
                }
                Payload::Approval(approval_ballot) => {
                    let approved: Vec<CandidateId> = approval_ballot
                        .approvals
                        .iter()
                        .filter_map(|name| {
                            candidate_map
                                .get(name.as_str())
                                .map(|&idx| CandidateId::new(idx, name.clone()))
                        })
                        .collect();
                    ballots.push(BallotData::Simple(approved));
                }
                Payload::Score(score_ballot) => {
                    let scores: Vec<(CandidateId, usize)> = score_ballot
                        .scores
                        .iter()
                        .filter_map(|entry| {
                            candidate_map.get(entry.candidate_id.as_str()).map(|&idx| {
                                (
                                    CandidateId::new(idx, entry.candidate_id.clone()),
                                    entry.value as usize,
                                )
                            })
                        })
                        .collect();
                    ballots.push(BallotData::Scoring(scores));
                }
            }
        }
    }

    Ok(ballots)
}

/// Form a response based on the header, ballot batches and the registry state of the service.
///
/// # Panics
///
/// Will panic if the system metrics couldn't be serialized.
/// Shouldn't happen as the type implement `Serialize` trait.
#[must_use]
pub fn process_request(
    header: &RunHeader,
    batches: &[BallotBatch],
    registry: &Registry,
) -> RunResult {
    let candidate_map: HashMap<&str, usize> = header
        .candidates
        .iter()
        .enumerate()
        .map(|(i, c)| (c.name.as_str(), i))
        .collect();

    let names: Vec<String> = header.candidates.iter().map(|c| c.name.clone()).collect();

    let ballots = match parse_ballots(batches, &candidate_map) {
        Ok(ballots) => ballots,
        Err(e) => return create_error_type(tonic::Code::InvalidArgument, e),
    };

    if header.ballot_format != "ranking"
        && header.ballot_format != "approval"
        && header.ballot_format != "scoring"
    {
        return create_error_type(tonic::Code::Unimplemented, "not yet supported");
    }

    let mut system_metrics = SystemMetricsCollector::new(ballots.len());

    #[allow(clippy::expect_used)]
    match registry.execute(
        ballots,
        names,
        header.tally_rule.as_str(),
        &header.ballot_format,
    ) {
        Ok(result) => {
            let timings_json =
                serde_json::to_vec(&system_metrics.measure()).expect("Serialization failed");
            create_winner_response(result.0, &result.1, &result.2, &timings_json)
        }
        Err(AlgorithmError::NoSuchAlgorithm(a)) => create_error_type(
            tonic::Code::Unimplemented,
            format!("No such algorithm: {a}"),
        ),
        Err(AlgorithmError::InvalidArgument(e) | AlgorithmError::InvalidBallotType(e)) => {
            create_error_type(tonic::Code::InvalidArgument, e)
        }
        Err(AlgorithmError::UnsupportedBallotForAlgorithm { algorithm, ballot }) => {
            create_error_type(
                tonic::Code::InvalidArgument,
                format!("Algorithm {algorithm} does not support ballot type {ballot}"),
            )
        }
    }
}

/// Test helpers for constructing proto messages.
pub mod test_helpers {
    use crate::securevoting::compute::v1::{
        ApprovalBallot, Ballot, BallotBatch, Candidate, RankingBallot, RunHeader, ScoreBallot,
        ScoreEntry, ballot::Payload,
    };

    /// Create a ranking ballot header.
    #[must_use]
    pub fn ranking_header(candidates: &[&str]) -> RunHeader {
        RunHeader {
            tally_rule: "plurality".into(),
            ballot_format: "ranking".into(),
            candidates: candidates
                .iter()
                .map(|name| Candidate {
                    id: (*name).into(),
                    name: (*name).into(),
                })
                .collect(),
            ..Default::default()
        }
    }

    /// Create an approval ballot header.
    #[must_use]
    pub fn approval_header(candidates: &[&str]) -> RunHeader {
        RunHeader {
            tally_rule: "approval-2".into(),
            ballot_format: "approval".into(),
            candidates: candidates
                .iter()
                .map(|name| Candidate {
                    id: (*name).into(),
                    name: (*name).into(),
                })
                .collect(),
            ..Default::default()
        }
    }

    /// Create a scoring ballot header.
    #[must_use]
    pub fn scoring_header(candidates: &[&str]) -> RunHeader {
        RunHeader {
            tally_rule: "score".into(),
            ballot_format: "scoring".into(),
            candidates: candidates
                .iter()
                .map(|name| Candidate {
                    id: (*name).into(),
                    name: (*name).into(),
                })
                .collect(),
            ..Default::default()
        }
    }

    /// Create a ranking ballot.
    #[must_use]
    pub fn ranking_ballot(ranking: Vec<&str>) -> Ballot {
        Ballot {
            voter_ref: String::new(),
            payload: Some(Payload::Ranking(RankingBallot {
                ranking: ranking.into_iter().map(String::from).collect(),
            })),
        }
    }

    /// Create an approval ballot.
    #[must_use]
    pub fn approval_ballot(approvals: Vec<&str>) -> Ballot {
        Ballot {
            voter_ref: String::new(),
            payload: Some(Payload::Approval(ApprovalBallot {
                approvals: approvals.into_iter().map(String::from).collect(),
            })),
        }
    }

    /// Create a scoring ballot.
    #[must_use]
    pub fn scoring_ballot(scores: Vec<(&str, i32)>) -> Ballot {
        Ballot {
            voter_ref: String::new(),
            payload: Some(Payload::Score(ScoreBallot {
                scores: scores
                    .into_iter()
                    .map(|(candidate_id, value)| ScoreEntry {
                        candidate_id: candidate_id.into(),
                        value,
                    })
                    .collect(),
            })),
        }
    }

    /// Create a ballot batch.
    #[must_use]
    pub fn batch(ballots: Vec<Ballot>) -> BallotBatch {
        BallotBatch { ballots }
    }
}
