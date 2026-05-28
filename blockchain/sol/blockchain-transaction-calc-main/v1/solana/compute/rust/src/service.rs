use crate::compute;
use crate::error::ComputeError;
use crate::proto::solana::v1::compute_service_server::ComputeService;
use crate::proto::solana::v1::{EvaluateSwapRequest, EvaluateSwapResponse};
use crate::types::ComputeRequest;

use tonic::{Request, Response, Status};

#[derive(Debug, Default)]
pub struct ComputeServiceImpl;

#[tonic::async_trait]
impl ComputeService for ComputeServiceImpl {
    async fn evaluate_swap(
        &self,
        request: Request<EvaluateSwapRequest>,
    ) -> Result<Response<EvaluateSwapResponse>, Status> {
        let internal = ComputeRequest::try_from(request.into_inner())
            .map_err(|error| status_for_error(error))?;
        let response = compute::evaluate(internal)
            .map(EvaluateSwapResponse::from)
            .map_err(|error| status_for_error(error))?;

        Ok(Response::new(response))
    }
}

fn status_for_error(error: ComputeError) -> Status {
    match error {
        ComputeError::InvalidRequest => Status::invalid_argument(error.to_string()),
        ComputeError::TooManyRouteCandidates | ComputeError::RouteHopCountTooLarge => {
            Status::resource_exhausted(error.to_string())
        }
    }
}
