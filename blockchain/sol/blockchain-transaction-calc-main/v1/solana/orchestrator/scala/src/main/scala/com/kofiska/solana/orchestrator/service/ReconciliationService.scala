package com.kofiska.solana.orchestrator.service

import com.kofiska.solana.orchestrator.ports.{AuditPublisher, DecisionRepository, DedupeCache}

import scala.concurrent.{ExecutionContext, Future}

final class ReconciliationService(
  decisionRepository: DecisionRepository,
  dedupeCache: DedupeCache,
  auditPublisher: AuditPublisher,
  dedupeTtlSeconds: Long
)(implicit ec: ExecutionContext) {

  def runOnce(limit: Int): Future[ReconciliationReport] =
    for {
      pending <- decisionRepository.pendingAudit(limit)
      auditRepublished <- publishPendingAudit(pending)
      dedupeDrift <- repairDedupe(limit)
    } yield ReconciliationReport(
      pendingAuditCount = pending.size,
      republishedAuditCount = auditRepublished,
      dedupeRepairedCount = dedupeDrift
    )

  private def publishPendingAudit(events: Vector[com.kofiska.solana.orchestrator.domain.TransitionEvent]): Future[Int] =
    events.foldLeft(Future.successful(0)) { (acc, event) =>
      acc.flatMap { count =>
        auditPublisher.publish(event).flatMap { _ =>
          decisionRepository.markAuditPublished(event.requestId, event.decisionId).map(_ => count + 1)
        }.recover { case _ => count }
      }
    }

  private def repairDedupe(limit: Int): Future[Int] =
    for {
      keys <- dedupeCache.scan("solana:dedupe:", limit)
      repaired <- keys.foldLeft(Future.successful(0)) { (acc, key) =>
        acc.flatMap { count =>
          val requestId = key.stripPrefix("solana:dedupe:")
          decisionRepository.find(requestId).flatMap {
            case Some(result) =>
              dedupeCache.put(requestId, result.decisionId, dedupeTtlSeconds).map(_ => count + 1)
            case None =>
              dedupeCache.delete(requestId).map(_ => count)
          }
        }
      }
    } yield repaired
}

final case class ReconciliationReport(
  pendingAuditCount: Int,
  republishedAuditCount: Int,
  dedupeRepairedCount: Int
)
