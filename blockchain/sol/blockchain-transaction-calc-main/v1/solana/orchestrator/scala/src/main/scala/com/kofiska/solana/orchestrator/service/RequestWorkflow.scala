package com.kofiska.solana.orchestrator.service

import com.kofiska.solana.orchestrator.domain._
import com.kofiska.solana.orchestrator.ports.{AuditPublisher, ComputeGateway, DecisionRepository, DedupeCache}
import com.kofiska.solana.v1.{Actionability => ProtoActionability, EvaluateSwapResponse, TerminalState => ProtoTerminalState}

import java.util.UUID
import scala.concurrent.{ExecutionContext, Future}
import scala.util.control.NonFatal

final class RequestWorkflow(
  computeGateway: ComputeGateway,
  decisionRepository: DecisionRepository,
  dedupeCache: DedupeCache,
  auditPublisher: AuditPublisher
)(implicit ec: ExecutionContext) {

  def process(ctx: RequestContext): Future[DecisionResult] = {
    dedupeCache.get(ctx.requestId).flatMap {
      case Some(_) =>
        decisionRepository.find(ctx.requestId).flatMap {
          case Some(result) =>
            auditPublisher.publish(replayEvent(ctx, result)).map(_ => result)
          case None =>
            val result = missingTerminalRecord(ctx)
            persistAndAudit(ctx, result, computeLatencyMs = 0L)
        }
      case None =>
        computeGateway.evaluate(ctx).recover {
          case NonFatal(error) => failedComputeResult(ctx, error.getMessage)
        }.flatMap { response =>
          val result = resultFromResponse(ctx, response)
          persistAndAudit(ctx, result, response.computeLatencyMs.toLong)
        }
    }
  }

  private def persistAndAudit(
    ctx: RequestContext,
    result: DecisionResult,
    computeLatencyMs: Long
  ): Future[DecisionResult] = {
    val event = transitionEvent(ctx, result, computeLatencyMs)
    for {
      _ <- decisionRepository.upsert(ctx, result)
      _ <- dedupeCache.put(ctx.requestId, result.decisionId, ttlSeconds = 3600)
      _ <- auditPublisher.publish(event)
    } yield result
  }

  private def resultFromResponse(ctx: RequestContext, response: EvaluateSwapResponse): DecisionResult = {
    val terminalState = response.terminalState match {
      case ProtoTerminalState.ACCEPT => TerminalState.Accept
      case ProtoTerminalState.DEFER   => TerminalState.Defer
      case ProtoTerminalState.REJECT  => TerminalState.Reject
      case ProtoTerminalState.FAILED  => TerminalState.Failed
      case _                          => TerminalState.Failed
    }

    val actionability = response.actionability match {
      case ProtoActionability.ACTIONABLE    => Actionability.Actionable
      case ProtoActionability.NON_ACTIONABLE => Actionability.NonActionable
      case ProtoActionability.STALE         => Actionability.Stale
      case ProtoActionability.CONFLICT      => Actionability.Conflict
      case _                                => Actionability.NonActionable
    }

    DecisionResult(
      requestId = response.requestId,
      decisionId = if (response.decisionId.nonEmpty) response.decisionId else UUID.randomUUID().toString,
      terminalState = terminalState,
      reasonCode = response.reasonCode,
      actionability = actionability,
      bestRouteId = Option(response.bestRouteId).filter(_.nonEmpty),
      expectedOutput = Option(response.expectedOutput).filter(_.nonEmpty),
      feeCost = Option(response.feeCost).filter(_.nonEmpty),
      slippageCost = Option(response.slippageCost).filter(_.nonEmpty),
      breakevenMargin = Option(response.breakevenMargin).filter(_.nonEmpty),
      evEstimate = Option(response.evEstimate).filter(_.nonEmpty),
      evLowerBound = Option(response.evLowerBound).filter(_.nonEmpty),
      riskScore = Option(response.riskScore).filter(_.nonEmpty),
      freshnessValid = response.freshnessValid
    )
  }

  private def missingTerminalRecord(ctx: RequestContext): DecisionResult =
    DecisionResult(
      requestId = ctx.requestId,
      decisionId = UUID.randomUUID().toString,
      terminalState = TerminalState.Failed,
      reasonCode = "DEDUPED_BUT_MISSING_TERMINAL_RECORD",
      actionability = Actionability.NonActionable,
      bestRouteId = ctx.routeId,
      expectedOutput = None,
      feeCost = None,
      slippageCost = None,
      breakevenMargin = None,
      evEstimate = None,
      evLowerBound = None,
      riskScore = None,
      freshnessValid = false
    )

  private def failedComputeResult(ctx: RequestContext, reason: String): DecisionResult =
    DecisionResult(
      requestId = ctx.requestId,
      decisionId = UUID.randomUUID().toString,
      terminalState = TerminalState.Failed,
      reasonCode = if (reason.nonEmpty) reason else "COMPUTE_FAILED",
      actionability = Actionability.NonActionable,
      bestRouteId = ctx.routeId,
      expectedOutput = None,
      feeCost = None,
      slippageCost = None,
      breakevenMargin = None,
      evEstimate = None,
      evLowerBound = None,
      riskScore = None,
      freshnessValid = false
    )

  private def transitionEvent(
    ctx: RequestContext,
    result: DecisionResult,
    computeLatencyMs: Long
  ): TransitionEvent =
    TransitionEvent(
      traceId = ctx.traceId,
      requestId = ctx.requestId,
      decisionId = result.decisionId,
      terminalState = TerminalState.asString(result.terminalState),
      reasonCode = result.reasonCode,
      modelVersion = ctx.modelVersion,
      routeId = result.bestRouteId,
      slot = ctx.slot,
      quoteAge = ctx.quoteAge,
      sourceHashes = ctx.sourceHashes,
      stage = "terminal",
      latencyMs = computeLatencyMs,
      bytesIn = 0L,
      bytesOut = 0L,
      success = result.terminalState != TerminalState.Failed
    )

  private def replayEvent(ctx: RequestContext, result: DecisionResult): TransitionEvent =
    TransitionEvent(
      traceId = ctx.traceId,
      requestId = ctx.requestId,
      decisionId = result.decisionId,
      terminalState = TerminalState.asString(result.terminalState),
      reasonCode = "REPLAY_HIT",
      modelVersion = ctx.modelVersion,
      routeId = result.bestRouteId,
      slot = ctx.slot,
      quoteAge = ctx.quoteAge,
      sourceHashes = ctx.sourceHashes,
      stage = "replay",
      latencyMs = 0L,
      bytesIn = 0L,
      bytesOut = 0L,
      success = true
    )
}
