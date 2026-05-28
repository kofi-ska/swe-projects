pub fn risk_score(hop_count: u32, venue: &str) -> f64 {
    let hop_risk = f64::from(hop_count) * 0.0012;
    let venue_risk = venue_risk(venue);
    (hop_risk + venue_risk).clamp(0.0, 1.0)
}

fn venue_risk(venue: &str) -> f64 {
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
