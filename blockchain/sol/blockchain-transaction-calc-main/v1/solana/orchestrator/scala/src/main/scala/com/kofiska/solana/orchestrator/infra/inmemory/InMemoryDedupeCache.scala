package com.kofiska.solana.orchestrator.infra.inmemory

import com.kofiska.solana.orchestrator.ports.DedupeCache

import java.time.Instant
import scala.collection.concurrent.TrieMap
import scala.concurrent.{ExecutionContext, Future}

final class InMemoryDedupeCache(implicit ec: ExecutionContext) extends DedupeCache {
  private case class Entry(decisionId: String, expiresAt: Long)
  private val entries = TrieMap.empty[String, Entry]

  override def get(requestId: String): Future[Option[String]] =
    Future.successful {
      entries.get(requestId).filter(_.expiresAt > Instant.now().getEpochSecond).map(_.decisionId)
    }

  override def put(requestId: String, decisionId: String, ttlSeconds: Long): Future[Unit] =
    Future {
      entries.put(requestId, Entry(decisionId, Instant.now().getEpochSecond + ttlSeconds))
      ()
    }
}
