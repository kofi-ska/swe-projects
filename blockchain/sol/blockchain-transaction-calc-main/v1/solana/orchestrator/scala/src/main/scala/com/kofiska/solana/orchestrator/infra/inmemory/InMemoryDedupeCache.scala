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
      purgeExpired(requestId)
      entries.get(requestId).filter(_.expiresAt > Instant.now().getEpochSecond).map(_.decisionId)
    }

  override def claim(requestId: String, marker: String, ttlSeconds: Long): Future[Boolean] =
    Future.successful {
      purgeExpired(requestId)
      val expiresAt = Instant.now().getEpochSecond + ttlSeconds
      entries.putIfAbsent(requestId, Entry(marker, expiresAt)).isEmpty
    }

  override def put(requestId: String, decisionId: String, ttlSeconds: Long): Future[Unit] =
    Future {
      entries.put(requestId, Entry(decisionId, Instant.now().getEpochSecond + ttlSeconds))
      ()
    }

  override def delete(requestId: String): Future[Unit] =
    Future {
      entries.remove(requestId)
      ()
    }

  override def scan(prefix: String, limit: Int): Future[Vector[String]] =
    Future.successful {
      entries.keys.take(limit).map(requestId => s"$prefix$requestId").toVector
    }

  private def purgeExpired(requestId: String): Unit =
    entries.get(requestId).foreach { entry =>
      if (entry.expiresAt <= Instant.now().getEpochSecond) {
        entries.remove(requestId, entry)
      }
    }
}
