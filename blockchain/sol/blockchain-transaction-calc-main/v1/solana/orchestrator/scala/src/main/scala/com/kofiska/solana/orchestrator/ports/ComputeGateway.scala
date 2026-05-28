package com.kofiska.solana.orchestrator.ports

import com.kofiska.solana.orchestrator.domain.RequestContext
import com.kofiska.solana.v1.EvaluateSwapResponse

import scala.concurrent.Future

trait ComputeGateway {
  def evaluate(ctx: RequestContext): Future[EvaluateSwapResponse]
}

