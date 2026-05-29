package com.kofiska.solana.orchestrator.domain

final case class RouteCandidateInput(
  routeId: String,
  venue: String,
  hopCount: Int
)

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
  sourceHashes: Vector[String],
  routeCandidates: Vector[RouteCandidateInput]
)

sealed trait TerminalState
object TerminalState {
  case object Accept extends TerminalState
  case object Defer extends TerminalState
  case object Reject extends TerminalState
  case object Failed extends TerminalState

  def fromString(value: String): TerminalState = value.toUpperCase match {
    case "ACCEPT" => Accept
    case "DEFER"  => Defer
    case "REJECT" => Reject
    case _        => Failed
  }

  def asString(value: TerminalState): String = value match {
    case Accept => "ACCEPT"
    case Defer  => "DEFER"
    case Reject => "REJECT"
    case Failed => "FAILED"
  }
}

sealed trait Actionability
object Actionability {
  case object Actionable extends Actionability
  case object NonActionable extends Actionability
  case object Stale extends Actionability
  case object Conflict extends Actionability

  def fromString(value: String): Actionability = value.toUpperCase match {
    case "ACTIONABLE"     => Actionable
    case "STALE"          => Stale
    case "CONFLICT"       => Conflict
    case _                 => NonActionable
  }

  def asString(value: Actionability): String = value match {
    case Actionable    => "ACTIONABLE"
    case NonActionable => "NON_ACTIONABLE"
    case Stale         => "STALE"
    case Conflict      => "CONFLICT"
  }
}

final case class DecisionResult(
  requestId: String,
  decisionId: String,
  terminalState: TerminalState,
  reasonCode: String,
  actionability: Actionability,
  bestRouteId: Option[String],
  sourceHashes: Vector[String],
  expectedOutput: Option[BigDecimal],
  feeCost: Option[BigDecimal],
  slippageCost: Option[BigDecimal],
  breakevenMargin: Option[BigDecimal],
  evEstimate: Option[BigDecimal],
  evLowerBound: Option[BigDecimal],
  riskScore: Option[BigDecimal],
  freshnessValid: Boolean
)
