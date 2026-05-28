package com.kofiska.solana.orchestrator.infra.inmemory

import com.kofiska.solana.orchestrator.domain.{DecisionResult, RequestContext}
import com.kofiska.solana.orchestrator.domain.TransitionEvent
import com.kofiska.solana.orchestrator.ports.DecisionRepository

import scala.collection.concurrent.TrieMap
import scala.concurrent.{ExecutionContext, Future}

final class InMemoryDecisionRepository(implicit ec: ExecutionContext) extends DecisionRepository {
  private val rows = TrieMap.empty[String, DecisionResult]
  private val auditRows = TrieMap.empty[(String, String), TransitionEvent]
  private val published = TrieMap.empty[(String, String), Boolean]

  override def find(requestId: String): Future[Option[DecisionResult]] =
    Future.successful(rows.get(requestId))

  override def upsert(ctx: RequestContext, result: DecisionResult, event: TransitionEvent): Future[Unit] =
    Future {
      rows.putIfAbsent(ctx.requestId, result)
      auditRows.putIfAbsent((ctx.requestId, result.decisionId), event)
      ()
    }

  override def pendingAudit(limit: Int): Future[Vector[TransitionEvent]] =
    Future.successful {
      auditRows.collect {
        case ((requestId, decisionId), event) if !published.contains((requestId, decisionId)) => event
      }.take(limit).toVector
    }

  override def markAuditPublished(requestId: String, decisionId: String): Future[Unit] =
    Future.successful {
      published.put((requestId, decisionId), true)
      ()
    }
}
