package com.kofiska.solana.orchestrator.clients

import com.kofiska.solana.v1.{EvaluateSwapRequest, EvaluateSwapResponse}

trait ComputeClient {
  def evaluate(request: EvaluateSwapRequest): scala.concurrent.Future[EvaluateSwapResponse]
}

