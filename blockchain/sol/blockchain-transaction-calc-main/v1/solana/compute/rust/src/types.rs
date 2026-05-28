#[derive(Debug, Clone)]
pub struct ComputeRequest {
    pub request_id: String,
    pub dedupe_key: String,
    pub trace_id: String,
    pub model_version: String,
}

#[derive(Debug, Clone)]
pub struct ComputeResponse {
    pub request_id: String,
    pub decision_id: String,
    pub terminal_state: String,
    pub reason_code: String,
}
