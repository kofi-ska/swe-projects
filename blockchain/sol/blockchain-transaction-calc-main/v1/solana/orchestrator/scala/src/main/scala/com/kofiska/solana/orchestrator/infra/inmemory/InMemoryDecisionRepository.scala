package com.kofiska.solana.orchestrator.infra.inmemory

import com.kofiska.solana.orchestrator.domain.{DecisionResult, RequestContext}
import com.kofiska.solana.orchestrator.ports.DecisionRepository

import scala.collection.concurrent.TrieMap
import scala.concurrent.{ExecutionContext, Future}

final class InMemoryDecisionRepository(implicit ec: ExecutionContext) extends DecisionRepository {
  private val rows = TrieMap.empty[String, DecisionResult]

  override def find(requestId: String): Future[Option[DecisionResult]] =
    Future.successful(rows.get(requestId))

  override def upsert(ctx: RequestContext, result: DecisionResult): Future[Unit] =
    Future {
      rows.put(ctx.requestId, result)
      ()
    }
}
