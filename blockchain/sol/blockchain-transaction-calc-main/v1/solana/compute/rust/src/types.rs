use crate::error::ComputeError;
use crate::proto::solana::v1::{
    Actionability, EvaluateSwapRequest, EvaluateSwapResponse, TerminalState,
};

#[derive(Debug, Clone)]
pub struct ComputeRouteCandidate {
    pub route_id: String,
    pub venue: String,
    pub hop_count: u32,
}

#[derive(Debug, Clone)]
pub struct ComputeRequest {
    pub request_id: String,
    pub dedupe_key: String,
    pub trace_id: String,
    pub model_version: String,
    pub token_in: String,
    pub token_out: String,
    pub amount_in: f64,
    pub route_id: Option<String>,
    pub slot: u64,
    pub quote_age: u32,
    pub source_hashes: Vec<String>,
    pub route_candidates: Vec<ComputeRouteCandidate>,
}

#[derive(Debug, Clone)]
pub struct ComputeResponse {
    pub request_id: String,
    pub decision_id: String,
    pub terminal_state: String,
    pub actionability: String,
    pub reason_code: String,
    pub best_route_id: Option<String>,
    pub expected_output: f64,
    pub fee_cost: f64,
    pub slippage_cost: f64,
    pub breakeven_margin: f64,
    pub ev_estimate: f64,
    pub ev_lower_bound: f64,
    pub risk_score: f64,
    pub freshness_valid: bool,
    pub compute_latency_ms: u64,
    pub source_hashes: Vec<String>,
}

impl TryFrom<EvaluateSwapRequest> for ComputeRequest {
    type Error = ComputeError;

    fn try_from(value: EvaluateSwapRequest) -> Result<Self, Self::Error> {
        let amount_in = value.amount_in.parse::<f64>().map_err(|_| ComputeError::InvalidRequest)?;
        let mut route_candidates = Vec::with_capacity(value.route_candidates.len());
        for candidate in value.route_candidates {
            route_candidates.push(ComputeRouteCandidate {
                route_id: candidate.route_id,
                venue: candidate.venue,
                hop_count: candidate.hop_count,
            });
        }

        Ok(Self {
            request_id: value.request_id,
            dedupe_key: value.dedupe_key,
            trace_id: value.trace_id,
            model_version: value.model_version,
            token_in: value.token_in,
            token_out: value.token_out,
            amount_in,
            route_id: if value.route_id.is_empty() { None } else { Some(value.route_id) },
            slot: value.slot,
            quote_age: value.quote_age,
            source_hashes: value.source_hashes,
            route_candidates,
        })
    }
}

impl From<ComputeResponse> for EvaluateSwapResponse {
    fn from(value: ComputeResponse) -> Self {
        EvaluateSwapResponse {
            request_id: value.request_id,
            decision_id: value.decision_id,
            terminal_state: match value.terminal_state.as_str() {
                "ACCEPT" => TerminalState::Accept as i32,
                "DEFER" => TerminalState::Defer as i32,
                "REJECT" => TerminalState::Reject as i32,
                "FAILED" => TerminalState::Failed as i32,
                _ => TerminalState::Failed as i32,
            },
            actionability: match value.actionability.as_str() {
                "ACTIONABLE" => Actionability::Actionable as i32,
                "STALE" => Actionability::Stale as i32,
                "CONFLICT" => Actionability::Conflict as i32,
                _ => Actionability::NonActionable as i32,
            },
            reason_code: value.reason_code,
            best_route_id: value.best_route_id.unwrap_or_default(),
            expected_output: format_amount(value.expected_output),
            fee_cost: format_amount(value.fee_cost),
            slippage_cost: format_amount(value.slippage_cost),
            breakeven_margin: format_amount(value.breakeven_margin),
            ev_estimate: format_amount(value.ev_estimate),
            ev_lower_bound: format_amount(value.ev_lower_bound),
            risk_score: format_amount(value.risk_score),
            freshness_valid: value.freshness_valid,
            compute_latency_ms: value.compute_latency_ms,
            source_hashes: value.source_hashes,
        }
    }
}

fn format_amount(value: f64) -> String {
    format!("{value:.6}")
}
