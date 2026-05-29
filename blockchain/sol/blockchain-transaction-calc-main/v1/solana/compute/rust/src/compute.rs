use crate::breakeven;
use crate::ev;
use crate::error::ComputeError;
use crate::fee;
use crate::freshness;
use crate::risk;
use crate::route_scoring::{self, RouteScore};
use crate::source_truth::{self, SourceTruthState};
use crate::slippage;
use crate::types::{ComputeRequest, ComputeResponse, ComputeRouteCandidate};

use std::time::Instant;
use tracing::debug;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum LifecycleStage {
    Received,
    Normalized,
    FreshnessChecked,
    SourceChecked,
    Routed,
    Scored,
    Terminal,
}

const MAX_ROUTE_CANDIDATES: usize = 16;
const MAX_HOP_COUNT: u32 = 8;

pub fn evaluate(request: ComputeRequest) -> Result<ComputeResponse, ComputeError> {
    let started = Instant::now();
    debug!(stage = ?LifecycleStage::Received, request_id = %request.request_id, "compute stage");

    let request = normalize(request)?;
    let mut stage = LifecycleStage::Normalized;
    debug!(?stage, request_id = %request.request_id, "compute stage");

    if !freshness::is_fresh(request.quote_age) {
        return Ok(terminal_response(
            &request,
            stage,
            started,
            None,
            DecisionKind::Defer,
            "STALE_QUOTE",
            false,
        )
        .build());
    }
    stage = LifecycleStage::FreshnessChecked;
    debug!(?stage, request_id = %request.request_id, "compute stage");

    match source_truth::evaluate(&request.source_hashes) {
        SourceTruthState::Missing => {
            return Ok(terminal_response(
                &request,
                stage,
                started,
                None,
                DecisionKind::Reject,
                "MISSING_SOURCE_HASHES",
                false,
            )
            .build());
        }
        SourceTruthState::Conflict => {
            return Ok(terminal_response(
                &request,
                stage,
                started,
                None,
                DecisionKind::Reject,
                "SOURCE_CONFLICT",
                false,
            )
            .build());
        }
        SourceTruthState::Valid => {}
    }
    stage = LifecycleStage::SourceChecked;
    debug!(?stage, request_id = %request.request_id, "compute stage");

    let route_pool = route_pool(&request);
    let selected = route_scoring::best_route(request.amount_in, route_pool.as_slice())
        .ok_or(ComputeError::InvalidRequest)?;
    stage = LifecycleStage::Routed;
    debug!(?stage, request_id = %request.request_id, "compute stage");

    let economics = economics(&request, &selected);
    stage = LifecycleStage::Scored;
    debug!(?stage, request_id = %request.request_id, "compute stage");

    let decision = decide(&request, &economics);
    stage = LifecycleStage::Terminal;
    debug!(?stage, request_id = %request.request_id, "compute stage");

    Ok(terminal_response(
        &request,
        stage,
        started,
        Some(&selected),
        decision.kind,
        decision.reason_code,
        true,
    )
    .with_economics(economics))
}

fn normalize(mut request: ComputeRequest) -> Result<ComputeRequest, ComputeError> {
    trim_required(&mut request.request_id)?;
    trim_required(&mut request.dedupe_key)?;
    trim_required(&mut request.trace_id)?;
    trim_required(&mut request.model_version)?;
    trim_required(&mut request.token_in)?;
    trim_required(&mut request.token_out)?;

    if !request.amount_in.is_finite() || request.amount_in <= 0.0 {
        return Err(ComputeError::InvalidRequest);
    }

    if request.route_candidates.len() > MAX_ROUTE_CANDIDATES {
        return Err(ComputeError::TooManyRouteCandidates);
    }

    if let Some(route_id) = request.route_id.as_mut() {
        trim_required(route_id)?;
    }

    let mut normalized_routes = Vec::with_capacity(request.route_candidates.len());
    for mut candidate in request.route_candidates.drain(..) {
        trim_required(&mut candidate.route_id)?;
        trim_required(&mut candidate.venue)?;
        if candidate.hop_count == 0 || candidate.hop_count > MAX_HOP_COUNT {
            return Err(ComputeError::RouteHopCountTooLarge);
        }
        normalized_routes.push(candidate);
    }
    request.route_candidates = normalized_routes;

    Ok(request)
}

fn trim_required(value: &mut String) -> Result<(), ComputeError> {
    let trimmed = value.trim().to_string();
    if trimmed.is_empty() {
        return Err(ComputeError::InvalidRequest);
    }
    *value = trimmed;
    Ok(())
}

enum RoutePool<'a> {
    Borrowed(&'a [ComputeRouteCandidate]),
    Owned(Vec<ComputeRouteCandidate>),
}

impl<'a> RoutePool<'a> {
    fn as_slice(&self) -> &[ComputeRouteCandidate] {
        match self {
            RoutePool::Borrowed(slice) => slice,
            RoutePool::Owned(routes) => routes.as_slice(),
        }
    }
}

fn route_pool(request: &ComputeRequest) -> RoutePool<'_> {
    if !request.route_candidates.is_empty() {
        return RoutePool::Borrowed(request.route_candidates.as_slice());
    }

    match request.route_id.as_ref() {
        Some(route_id) => RoutePool::Owned(vec![ComputeRouteCandidate {
            route_id: route_id.clone(),
            venue: "provided".to_string(),
            hop_count: 1,
        }]),
        None => RoutePool::Owned(Vec::new()),
    }
}

#[derive(Debug, Clone, Copy)]
struct Economics {
    expected_output: f64,
    fee_cost: f64,
    slippage_cost: f64,
    breakeven_margin: f64,
    ev_estimate: f64,
    ev_lower_bound: f64,
    risk_score: f64,
    freshness_valid: bool,
}

fn economics(request: &ComputeRequest, route: &RouteScore) -> Economics {
    let expected_output = route.expected_output;
    let fee_cost = fee::fee_cost(request.amount_in, route.hop_count);
    let slippage_cost = slippage::slippage_cost(expected_output, route.hop_count);
    let risk_score = risk::risk_score(route.hop_count, &route.venue);
    let breakeven_margin = breakeven::margin(expected_output, request.amount_in, fee_cost, slippage_cost);
    let ev_estimate = ev::estimate(expected_output, request.amount_in, fee_cost, slippage_cost, risk_score);
    let ev_lower_bound = ev::lower_bound(ev_estimate, uncertainty_margin(request.quote_age, risk_score));

    Economics {
        expected_output,
        fee_cost,
        slippage_cost,
        breakeven_margin,
        ev_estimate,
        ev_lower_bound,
        risk_score,
        freshness_valid: true,
    }
}

#[derive(Debug, Clone, Copy)]
struct Decision {
    kind: DecisionKind,
    reason_code: &'static str,
}

#[derive(Debug, Clone, Copy)]
enum DecisionKind {
    Accept,
    Defer,
    Reject,
}

fn decide(request: &ComputeRequest, economics: &Economics) -> Decision {
    if economics.ev_lower_bound <= 0.0 {
        return Decision {
            kind: DecisionKind::Reject,
            reason_code: "NON_POSITIVE_ECONOMICS",
        };
    }

    if economics.risk_score > 0.35 {
        return Decision {
            kind: DecisionKind::Reject,
            reason_code: "RISK_TOO_HIGH",
        };
    }

    if request.quote_age > freshness::MAX_QUOTE_AGE / 2 && economics.ev_lower_bound < 0.05 {
        return Decision {
            kind: DecisionKind::Defer,
            reason_code: "MARGINAL_AND_AGEING",
        };
    }

    Decision {
        kind: DecisionKind::Accept,
        reason_code: "ACCEPTED",
    }
}

fn uncertainty_margin(quote_age: u32, risk_score: f64) -> f64 {
    let age_margin = f64::from(quote_age) * 0.01;
    let risk_margin = risk_score * 0.05;
    age_margin + risk_margin
}

struct TerminalBuilder<'a> {
    request: &'a ComputeRequest,
    stage: LifecycleStage,
    started: Instant,
    route: Option<&'a RouteScore>,
    terminal_state: &'static str,
    actionability: &'static str,
    reason_code: &'static str,
    freshness_valid: bool,
    economics: Option<Economics>,
}

impl<'a> TerminalBuilder<'a> {
    fn with_economics(mut self, economics: Economics) -> ComputeResponse {
        self.economics = Some(economics);
        self.build()
    }

    fn build(self) -> ComputeResponse {
        debug!(?self.stage, request_id = %self.request.request_id, "compute terminal");
        let economics = self.economics.unwrap_or(Economics {
            expected_output: 0.0,
            fee_cost: 0.0,
            slippage_cost: 0.0,
            breakeven_margin: 0.0,
            ev_estimate: 0.0,
            ev_lower_bound: 0.0,
            risk_score: 0.0,
            freshness_valid: self.freshness_valid,
        });
        let decision_id = make_decision_id(
            &self.request.request_id,
            self.terminal_state,
            self.reason_code,
            self.route.map(|route| route.route_id.as_str()),
        );

        ComputeResponse {
            request_id: self.request.request_id.clone(),
            decision_id,
            terminal_state: self.terminal_state.to_string(),
            actionability: self.actionability.to_string(),
            reason_code: self.reason_code.to_string(),
            best_route_id: self.route.map(|route| route.route_id.clone()),
            expected_output: economics.expected_output,
            fee_cost: economics.fee_cost,
            slippage_cost: economics.slippage_cost,
            breakeven_margin: economics.breakeven_margin,
            ev_estimate: economics.ev_estimate,
            ev_lower_bound: economics.ev_lower_bound,
            risk_score: economics.risk_score,
            freshness_valid: economics.freshness_valid,
            compute_latency_ms: self.started.elapsed().as_millis() as u64,
            source_hashes: self.request.source_hashes.clone(),
        }
    }
}

fn terminal_response<'a>(
    request: &'a ComputeRequest,
    stage: LifecycleStage,
    started: Instant,
    route: Option<&'a RouteScore>,
    decision_kind: DecisionKind,
    reason_code: &'static str,
    freshness_valid: bool,
) -> TerminalBuilder<'a> {
    let terminal_state = match decision_kind {
        DecisionKind::Accept => "ACCEPT",
        DecisionKind::Defer => "DEFER",
        DecisionKind::Reject => "REJECT",
    };
    let actionability = match reason_code {
        "STALE_QUOTE" => "STALE",
        "SOURCE_CONFLICT" | "MISSING_SOURCE_HASHES" => "CONFLICT",
        "ACCEPTED" => "ACTIONABLE",
        _ => "NON_ACTIONABLE",
    };

    TerminalBuilder {
        request,
        stage,
        started,
        route,
        terminal_state,
        actionability,
        reason_code,
        freshness_valid,
        economics: None,
    }
}

fn make_decision_id(
    request_id: &str,
    terminal_state: &str,
    reason_code: &str,
    route_id: Option<&str>,
) -> String {
    match route_id {
        Some(route_id) => format!("{request_id}:{terminal_state}:{reason_code}:{route_id}"),
        None => format!("{request_id}:{terminal_state}:{reason_code}"),
    }
}
