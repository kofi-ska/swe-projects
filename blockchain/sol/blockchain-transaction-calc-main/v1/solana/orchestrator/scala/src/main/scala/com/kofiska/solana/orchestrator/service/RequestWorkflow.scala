package com.kofiska.solana.orchestrator.service

import com.kofiska.solana.orchestrator.domain.{DecisionResult, RequestContext, TerminalState, TransitionEvent}
import com.kofiska.solana.orchestrator.ports.{AuditPublisher, ComputeGateway, DecisionRepository, DedupeCache}
import com.kofiska.solana.v1.{Actionability, EvaluateSwapResponse, TerminalState => ProtoTerminalState}

import java.util.UUID
import scala.concurrent.{ExecutionContext, Future}

final class RequestWorkflow(
  computeGateway: ComputeGateway,
  decisionRepository: DecisionRepository,
  dedupeCache: DedupeCache,
  auditPublisher: AuditPublisher
)(implicit ec: ExecutionContext) {

  def process(ctx: RequestContext): Future[DecisionResult] = {
    dedupeCache.get(ctx.requestId).flatMap {
      case Some(decisionId) =>
        decisionRepository.find(ctx.requestId).map {
          case Some(result) => result
          case None =>
            DecisionResult(
              requestId = ctx.requestId,
              decisionId = decisionId,
              terminalState = TerminalState.Failed,
              reasonCode = "DEDUPED_BUT_MISSING_TERMINAL_RECORD",
              actionability = "NON_ACTIONABLE",
              bestRouteId = ctx.routeId,
              evEstimate = None,
              evLowerBound = None
            )
        }
      case None =>
        computeGateway.evaluate(ctx).flatMap { response =>
          val result = toDecisionResult(ctx, response)
          val event = toTransitionEvent(ctx, result, response)
          for {
            _ <- decisionRepository.upsert(ctx, result)
            _ <- dedupeCache.put(ctx.requestId, result.decisionId, ttlSeconds = 3600)
            _ <- auditPublisher.publish(event)
          } yield result
        }
    }
  }

  private def toDecisionResult(ctx: RequestContext, response: EvaluateSwapResponse): DecisionResult = {
    val terminalState = response.terminalState match {
      case ProtoTerminalState.ACCEPT  => TerminalState.Accept
      case ProtoTerminalState.DEFER   => TerminalState.Defer
      case ProtoTerminalState.REJECT  => TerminalState.Reject
      case ProtoTerminalState.FAILED  => TerminalState.Failed
      case _                          => TerminalState.Failed
    }

    DecisionResult(
      requestId = response.requestId,
      decisionId = if (response.decisionId.nonEmpty) response.decisionId else UUID.randomUUID().toString,
      terminalState = terminalState,
      reasonCode = response.reasonCode,
      actionability = response.actionability.name,
      bestRouteId = Option(response.bestRouteId).filter(_.nonEmpty),
      evEstimate = Option(response.evEstimate).filter(_.nonEmpty),
      evLowerBound = Option(response.evLowerBound).filter(_.nonEmpty)
    )
  }

  private def toTransitionEvent(
    ctx: RequestContext,
    result: DecisionResult,
    response: EvaluateSwapResponse
  ): TransitionEvent = {
    TransitionEvent(
      traceId = ctx.traceId,
      requestId = ctx.requestId,
      decisionId = result.decisionId,
      terminalState = result.terminalState.toString,
      reasonCode = result.reasonCode,
      modelVersion = ctx.modelVersion,
      routeId = result.bestRouteId,
      slot = ctx.slot,
      quoteAge = ctx.quoteAge,
      sourceHashes = ctx.sourceHashes,
      stage = "terminal",
      latencyMs = response.computeLatencyMs,
      bytesIn = 0L,
      bytesOut = 0L,
      success = true
    )
  }
}
