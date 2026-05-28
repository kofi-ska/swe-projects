use thiserror::Error;

#[derive(Debug, Error)]
pub enum ComputeError {
    #[error("invalid request")]
    InvalidRequest,
}
