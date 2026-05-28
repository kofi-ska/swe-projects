pub fn margin(expected_output: f64, amount_in: f64, fee_cost: f64, slippage_cost: f64) -> f64 {
    expected_output - amount_in - fee_cost - slippage_cost
}
