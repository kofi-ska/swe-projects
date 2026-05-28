package com.kofiska.solana.orchestrator.domain

final case class RequestContext(
  requestId: String,
  dedupeKey: String,
  traceId: String,
  modelVersion: String,
  tokenIn: String,
  tokenOut: String,
  amountIn: String,
  routeId: Option[String],
  slot: Long,
  quoteAge: Long,
  sourceHashes: Vector[String]
)

sealed trait TerminalState
object TerminalState {
  case object Accept extends TerminalState
  case object Defer extends TerminalState
  case object Reject extends TerminalState
  case object Failed extends TerminalState
}

final case class DecisionResult(
  requestId: String,
  decisionId: String,
  terminalState: TerminalState,
  reasonCode: String,
  actionability: String,
  bestRouteId: Option[String],
  evEstimate: Option[String],
  evLowerBound: Option[String]
)

