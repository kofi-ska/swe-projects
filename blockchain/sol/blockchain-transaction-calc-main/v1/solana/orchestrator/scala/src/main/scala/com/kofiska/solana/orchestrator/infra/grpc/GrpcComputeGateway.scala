package com.kofiska.solana.orchestrator.infra.grpc

import com.kofiska.solana.orchestrator.domain.RequestContext
import com.kofiska.solana.orchestrator.ports.ComputeGateway
import com.kofiska.solana.v1.{ComputeServiceGrpc, EvaluateSwapRequest, EvaluateSwapResponse, RouteCandidate}

import io.grpc.ManagedChannel
import scala.concurrent.Future

final class GrpcComputeGateway(channel: ManagedChannel) extends ComputeGateway {
  private val stub = ComputeServiceGrpc.stub(channel)

  override def evaluate(ctx: RequestContext): Future[EvaluateSwapResponse] = {
    val request = EvaluateSwapRequest(
      requestId = ctx.requestId,
      dedupeKey = ctx.dedupeKey,
      traceId = ctx.traceId,
      modelVersion = ctx.modelVersion,
      tokenIn = ctx.tokenIn,
      tokenOut = ctx.tokenOut,
      amountIn = ctx.amountIn,
      routeId = ctx.routeId.getOrElse(""),
      slot = ctx.slot,
      quoteAge = ctx.quoteAge,
      sourceHashes = ctx.sourceHashes,
      routeCandidates = Vector.empty[RouteCandidate]
    )

    stub.evaluateSwap(request)
  }
}
