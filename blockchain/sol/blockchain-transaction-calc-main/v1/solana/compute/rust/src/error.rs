use thiserror::Error;

#[derive(Debug, Error)]
pub enum ComputeError {
    #[error("invalid request")]
    InvalidRequest,
    #[error("too many route candidates")]
    TooManyRouteCandidates,
    #[error("route hop count too large")]
    RouteHopCountTooLarge,
}
