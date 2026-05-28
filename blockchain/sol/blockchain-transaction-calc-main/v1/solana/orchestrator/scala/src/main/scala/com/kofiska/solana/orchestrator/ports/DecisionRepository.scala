package com.kofiska.solana.orchestrator.ports

import com.kofiska.solana.orchestrator.domain.{DecisionResult, RequestContext}

import scala.concurrent.Future

trait DecisionRepository {
  def find(requestId: String): Future[Option[DecisionResult]]
  def upsert(ctx: RequestContext, result: DecisionResult): Future[Unit]
}

