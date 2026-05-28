use solana_compute::compute;
use solana_compute::freshness;
use solana_compute::proto::solana::v1::compute_service_server::ComputeService;
use solana_compute::proto::solana::v1::{Actionability, EvaluateSwapRequest, TerminalState};
use solana_compute::route_scoring;
use solana_compute::service::ComputeServiceImpl;
use solana_compute::source_truth::{self, SourceTruthState};
use solana_compute::types::{ComputeRequest, ComputeRouteCandidate};
use tonic::Request;

fn base_request() -> ComputeRequest {
    ComputeRequest {
        request_id: "req-1".to_string(),
        dedupe_key: "dedupe-1".to_string(),
        trace_id: "trace-1".to_string(),
        model_version: "v1".to_string(),
        token_in: "USDC".to_string(),
        token_out: "SOL".to_string(),
        amount_in: 1_000.0,
        route_id: Some("route-a".to_string()),
        slot: 100,
        quote_age: 1,
        source_hashes: vec!["hash-a".to_string(), "hash-b".to_string()],
        route_candidates: vec![
            ComputeRouteCandidate {
                route_id: "route-a".to_string(),
                venue: "direct".to_string(),
                hop_count: 1,
            },
            ComputeRouteCandidate {
                route_id: "route-b".to_string(),
                venue: "aggregator".to_string(),
                hop_count: 4,
            },
        ],
    }
}

#[test]
fn freshness_gate_is_inclusive_at_threshold() {
    assert!(freshness::is_fresh(freshness::MAX_QUOTE_AGE));
    assert!(!freshness::is_fresh(freshness::MAX_QUOTE_AGE + 1));
}

#[test]
fn source_truth_detects_missing_and_conflicting_state() {
    assert_eq!(source_truth::evaluate(&[]), SourceTruthState::Missing);
    assert_eq!(
        source_truth::evaluate(&["hash-a".to_string(), "hash-a".to_string()]),
        SourceTruthState::Conflict
    );
    assert_eq!(
        source_truth::evaluate(&["hash-a".to_string(), "hash-b".to_string()]),
        SourceTruthState::Valid
    );
}

#[test]
fn route_scoring_prefers_the_better_route() {
    let request = base_request();
    let best = route_scoring::best_route(request.amount_in, request.route_candidates.as_slice())
        .expect("best route");

    assert_eq!(best.route_id, "route-a");
    assert!(best.score > 0.0);
    assert!(best.expected_output > request.amount_in);
}

#[tokio::test]
async fn stale_quotes_defer_before_economics() {
    let mut request = base_request();
    request.quote_age = freshness::MAX_QUOTE_AGE + 1;

    let response = compute::evaluate(request).expect("response");

    assert_eq!(response.terminal_state, "DEFER");
    assert_eq!(response.actionability, "STALE");
    assert_eq!(response.reason_code, "STALE_QUOTE");
    assert!(!response.freshness_valid);
}

#[tokio::test]
async fn conflicting_sources_reject() {
    let mut request = base_request();
    request.source_hashes = vec!["hash-a".to_string(), "hash-a".to_string()];

    let response = compute::evaluate(request).expect("response");

    assert_eq!(response.terminal_state, "REJECT");
    assert_eq!(response.actionability, "CONFLICT");
    assert_eq!(response.reason_code, "SOURCE_CONFLICT");
}

#[tokio::test]
async fn positive_routes_accept_and_negative_routes_reject() {
    let accept = compute::evaluate(base_request()).expect("accept response");
    assert_eq!(accept.terminal_state, "ACCEPT");
    assert_eq!(accept.actionability, "ACTIONABLE");
    assert!(accept.ev_lower_bound > 0.0);

    let mut reject = base_request();
    reject.route_candidates = vec![ComputeRouteCandidate {
        route_id: "route-c".to_string(),
        venue: "aggregator".to_string(),
        hop_count: 12,
    }];

    let response = compute::evaluate(reject).expect("reject response");
    assert_eq!(response.terminal_state, "REJECT");
    assert_eq!(response.actionability, "NON_ACTIONABLE");
    assert_eq!(response.reason_code, "NON_POSITIVE_ECONOMICS");
    assert!(response.ev_lower_bound <= 0.0);
}

#[tokio::test]
async fn grpc_service_matches_compute_engine() {
    let request = EvaluateSwapRequest {
        request_id: "req-2".to_string(),
        dedupe_key: "dedupe-2".to_string(),
        trace_id: "trace-2".to_string(),
        model_version: "v1".to_string(),
        token_in: "USDC".to_string(),
        token_out: "SOL".to_string(),
        amount_in: "1000".to_string(),
        route_id: "route-a".to_string(),
        slot: 100,
        quote_age: 1,
        source_hashes: vec!["hash-a".to_string(), "hash-b".to_string()],
        route_candidates: vec![
            solana_compute::proto::solana::v1::RouteCandidate {
                route_id: "route-a".to_string(),
                venue: "direct".to_string(),
                hop_count: 1,
            },
            solana_compute::proto::solana::v1::RouteCandidate {
                route_id: "route-b".to_string(),
                venue: "aggregator".to_string(),
                hop_count: 4,
            },
        ],
    };

    let response = ComputeServiceImpl::default()
        .evaluate_swap(Request::new(request))
        .await
        .expect("grpc response")
        .into_inner();

    assert_eq!(response.terminal_state, TerminalState::Accept as i32);
    assert_eq!(response.actionability, Actionability::Actionable as i32);
    assert_eq!(response.request_id, "req-2");
}
