pub fn slippage_cost(expected_output: f64, hop_count: u32) -> f64 {
    let base_rate = 0.0009;
    let hop_rate = f64::from(hop_count) * 0.00015;
    expected_output * (base_rate + hop_rate)
}
