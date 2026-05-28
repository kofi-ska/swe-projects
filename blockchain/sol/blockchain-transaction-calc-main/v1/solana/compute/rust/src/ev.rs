pub fn estimate(
    expected_output: f64,
    amount_in: f64,
    fee_cost: f64,
    slippage_cost: f64,
    risk_score: f64,
) -> f64 {
    expected_output - amount_in - fee_cost - slippage_cost - (amount_in * risk_score)
}

pub fn lower_bound(ev_estimate: f64, uncertainty_margin: f64) -> f64 {
    ev_estimate - uncertainty_margin
}
