pub fn fee_cost(amount_in: f64, hop_count: u32) -> f64 {
    let base_fee = amount_in * 0.0007;
    let hop_fee = amount_in * f64::from(hop_count) * 0.00008;
    base_fee + hop_fee
}
