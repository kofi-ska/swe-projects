package com.kofiska.solana.orchestrator.domain

final case class TransitionEvent(
  schemaVersion: String,
  traceId: String,
  requestId: String,
  decisionId: String,
  terminalState: String,
  reasonCode: String,
  modelVersion: String,
  routeId: Option[String],
  slot: Long,
  quoteAge: Long,
  sourceHashes: Vector[String],
  stage: String,
  latencyMs: Long,
  bytesIn: Long,
  bytesOut: Long,
  success: Boolean
)
