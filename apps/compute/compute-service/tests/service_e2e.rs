use compute_service::parse_ballots;
use compute_service::registry::voting_rules::get_core_registry;
use compute_service::test_helpers::{
    approval_ballot, approval_header, batch, ranking_ballot, ranking_header, scoring_ballot,
    scoring_header,
};
use compute_service::{process_request, registry::Registry};

fn registry() -> Registry {
    get_core_registry()
}

#[allow(clippy::unwrap_used)]
#[test]
fn ranking_ballot_returns_done() {
    let header = ranking_header(&["A", "B", "C"]);
    let batches = vec![batch(vec![
        ranking_ballot(vec!["A", "B", "C"]),
        ranking_ballot(vec!["A", "C", "B"]),
        ranking_ballot(vec!["B", "A", "C"]),
    ])];

    let result = process_request(&header, &batches, &registry());
    assert_eq!(result.status, "done");

    let winners: Vec<String> = serde_json::from_slice(&result.winners_json).unwrap();
    assert_eq!(winners, vec!["A"]);
}

#[test]
fn approval_ballot_returns_done() {
    // Approval Q=2: each ballot has Q approvals, n_candidates >= Q
    let header = approval_header(&["A", "B", "C"]);
    let batches = vec![batch(vec![
        approval_ballot(vec!["A", "B"]),
        approval_ballot(vec!["B", "C"]),
        approval_ballot(vec!["A", "C"]),
    ])];

    let result = process_request(&header, &batches, &registry());
    assert_eq!(result.status, "done", "error: {}", result.error_text);
}

#[allow(clippy::unwrap_used)]
#[test]
fn scoring_ballot_returns_done() {
    let header = scoring_header(&["A", "B", "C"]);
    let batches = vec![batch(vec![
        scoring_ballot(vec![("A", 5), ("B", 3), ("C", 1)]),
        scoring_ballot(vec![("A", 5), ("B", 3), ("C", 1)]),
        scoring_ballot(vec![("A", 5), ("B", 2), ("C", 3)]),
    ])];

    let result = process_request(&header, &batches, &registry());
    assert_eq!(result.status, "done");

    let winners: Vec<String> = serde_json::from_slice(&result.winners_json).unwrap();
    assert_eq!(winners, vec!["A"]);
}

#[test]
fn empty_batches_returns_error() {
    let header = ranking_header(&["A", "B"]);
    let batches: Vec<_> = vec![];

    let result = process_request(&header, &batches, &registry());
    assert_eq!(result.status, "error");
}

#[test]
fn empty_ballot_payload_returns_error() {
    use compute_service::securevoting::compute::v1::Ballot;

    let header = ranking_header(&["A", "B"]);
    let batches = vec![batch(vec![Ballot {
        voter_ref: String::new(),
        payload: None,
    }])];

    let result = process_request(&header, &batches, &registry());
    assert_eq!(result.status, "error");
    assert!(result.error_text.contains("empty ballot paylod"));
}

#[test]
fn unknown_ballot_format_returns_error() {
    use compute_service::securevoting::compute::v1::Candidate;

    let header = compute_service::securevoting::compute::v1::RunHeader {
        tally_rule: "plurality".into(),
        ballot_format: "quadratic".into(),
        candidates: ["A", "B"]
            .iter()
            .map(|name| Candidate {
                id: (*name).into(),
                name: (*name).into(),
            })
            .collect(),
        ..Default::default()
    };
    let batches = vec![batch(vec![ranking_ballot(vec!["A", "B"])])];

    let result = process_request(&header, &batches, &registry());
    assert_eq!(result.status, "error");
    assert!(result.error_text.contains("not yet supported"));
}

#[test]
fn unknown_algorithm_returns_error() {
    use compute_service::securevoting::compute::v1::Candidate;

    let header = compute_service::securevoting::compute::v1::RunHeader {
        tally_rule: "fantasy-voting".into(),
        ballot_format: "ranking".into(),
        candidates: ["A", "B"]
            .iter()
            .map(|name| Candidate {
                id: (*name).into(),
                name: (*name).into(),
            })
            .collect(),
        ..Default::default()
    };
    let batches = vec![batch(vec![ranking_ballot(vec!["A", "B"])])];

    let result = process_request(&header, &batches, &registry());
    assert_eq!(result.status, "error");
    assert!(result.error_text.contains("No such algorithm"));
}

#[test]
fn wrong_ballot_type_for_algorithm_returns_error() {
    // Score rule is only registered for Scoring ballot type.
    // Using it with ranking ballots should trigger UnsupportedBallotForAlgorithm.
    let header = ranking_header(&["A", "B"]);
    let mut header = header;
    header.tally_rule = "score".into();
    let batches = vec![batch(vec![ranking_ballot(vec!["A", "B"])])];

    let result = process_request(&header, &batches, &registry());
    assert_eq!(result.status, "error");
    assert!(result.error_text.contains("does not support ballot type"));
}

#[allow(clippy::unwrap_used)]
#[test]
fn multiple_batches_accumulated() {
    let header = ranking_header(&["A", "B"]);
    let batches = vec![
        batch(vec![ranking_ballot(vec!["A", "B"])]),
        batch(vec![ranking_ballot(vec!["A", "B"])]),
        batch(vec![ranking_ballot(vec!["B", "A"])]),
    ];

    let result = process_request(&header, &batches, &registry());
    assert_eq!(result.status, "done");

    let winners: Vec<String> = serde_json::from_slice(&result.winners_json).unwrap();
    assert_eq!(winners, vec!["A"]);
}

#[test]
fn metrics_and_protocol_populated_on_success() {
    let header = ranking_header(&["A", "B"]);
    let batches = vec![batch(vec![
        ranking_ballot(vec!["A", "B"]),
        ranking_ballot(vec!["A", "B"]),
    ])];

    let result = process_request(&header, &batches, &registry());
    assert_eq!(result.status, "done");
    assert!(!result.metrics_json.is_empty());
    assert!(!result.protocol_json.is_empty());
}

#[allow(clippy::unwrap_used)]
#[test]
fn parse_ballots_empty_batch_returns_empty() {
    let candidate_map = [("A", 0usize), ("B", 1usize)].into_iter().collect();
    let result = parse_ballots(&[], &candidate_map).unwrap();
    assert!(result.is_empty());
}

#[allow(clippy::unwrap_used)]
#[test]
fn parse_ballots_ranking() {
    let candidate_map = [("A", 0usize), ("B", 1usize), ("C", 2usize)]
        .into_iter()
        .collect();
    let batches = vec![batch(vec![ranking_ballot(vec!["A", "B", "C"])])];
    let result = parse_ballots(&batches, &candidate_map).unwrap();
    assert_eq!(result.len(), 1);
}

#[allow(clippy::unwrap_used)]
#[test]
fn parse_ballots_approval() {
    let candidate_map = [("A", 0usize), ("B", 1usize)].into_iter().collect();
    let batches = vec![batch(vec![approval_ballot(vec!["A", "B"])])];
    let result = parse_ballots(&batches, &candidate_map).unwrap();
    assert_eq!(result.len(), 1);
}

#[allow(clippy::unwrap_used)]
#[test]
fn parse_ballots_scoring() {
    let candidate_map = [("A", 0usize), ("B", 1usize)].into_iter().collect();
    let batches = vec![batch(vec![scoring_ballot(vec![("A", 5), ("B", 3)])])];
    let result = parse_ballots(&batches, &candidate_map).unwrap();
    assert_eq!(result.len(), 1);
}
