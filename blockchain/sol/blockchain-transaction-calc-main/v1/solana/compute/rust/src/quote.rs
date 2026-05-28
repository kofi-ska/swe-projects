use crate::types::ComputeRouteCandidate;

pub fn expected_output(amount_in: f64, route: &ComputeRouteCandidate) -> f64 {
    let base_efficiency = 1.0180;
    let hop_penalty = f64::from(route.hop_count) * 0.0014;
    let venue_penalty = venue_penalty(&route.venue);
    let efficiency = (base_efficiency - hop_penalty - venue_penalty).clamp(0.92, 1.03);
    amount_in * efficiency
}

fn venue_penalty(venue: &str) -> f64 {
    let normalized = venue.to_ascii_lowercase();
    if normalized.contains("aggregator") {
        0.0010
    } else if normalized.contains("amm") {
        0.0008
    } else if normalized.contains("pool") {
        0.0006
    } else {
        0.0004
    }
}
