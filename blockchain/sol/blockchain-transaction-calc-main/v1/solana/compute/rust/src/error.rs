use thiserror::Error;

#[derive(Debug, Error)]
pub enum ComputeError {
    #[error("stale data")]
    StaleData,
    #[error("source conflict")]
    SourceConflict,
    #[error("non-positive EV")]
    NonPositiveEv,
    #[error("invalid request")]
    InvalidRequest,
}
