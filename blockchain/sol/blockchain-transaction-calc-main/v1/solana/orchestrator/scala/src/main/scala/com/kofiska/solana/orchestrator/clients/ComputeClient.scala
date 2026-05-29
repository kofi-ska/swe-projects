package com.kofiska.solana.orchestrator.clients

import com.kofiska.solana.v1.decision.{EvaluateSwapRequest, EvaluateSwapResponse}

trait ComputeClient {
  def evaluate(request: EvaluateSwapRequest): scala.concurrent.Future[EvaluateSwapResponse]
}
