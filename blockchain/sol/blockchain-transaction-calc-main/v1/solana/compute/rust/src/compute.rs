pub mod breakeven;
pub mod ev;
pub mod fee;
pub mod freshness;
pub mod quote;
pub mod risk;
pub mod route_scoring;
pub mod slippage;
pub mod source_truth;
pub mod types;

pub fn compute_scaffold() -> &'static str {
    "compute"
}
