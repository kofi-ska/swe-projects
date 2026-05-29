use crate::quote;
use crate::types::ComputeRouteCandidate;

#[derive(Debug, Clone)]
pub struct RouteScore {
    pub route_id: String,
    pub venue: String,
    pub hop_count: u32,
    pub score: f64,
    pub expected_output: f64,
    pub fee_cost: f64,
    pub slippage_cost: f64,
    pub risk_score: f64,
}

pub fn best_route(amount_in: f64, routes: &[ComputeRouteCandidate]) -> Option<RouteScore> {
    routes
        .iter()
        .map(|route| score_route(amount_in, route))
        .max_by(|left, right| {
            left.score
                .total_cmp(&right.score)
                .then_with(|| right.hop_count.cmp(&left.hop_count))
                .then_with(|| right.route_id.cmp(&left.route_id))
        })
}

pub fn score_route(amount_in: f64, route: &ComputeRouteCandidate) -> RouteScore {
    let expected_output = quote::expected_output(amount_in, route);
    let fee_cost = crate::fee::fee_cost(amount_in, route.hop_count);
    let slippage_cost = crate::slippage::slippage_cost(expected_output, route.hop_count);
    let risk_score = crate::risk::risk_score(route.hop_count, &route.venue);
    let score = expected_output - fee_cost - slippage_cost - (risk_score * amount_in);

    RouteScore {
        route_id: route.route_id.clone(),
        venue: route.venue.clone(),
        hop_count: route.hop_count,
        score,
        expected_output,
        fee_cost,
        slippage_cost,
        risk_score,
    }
}
