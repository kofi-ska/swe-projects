package com.kofiska.solana.orchestrator.infra.inmemory

import com.kofiska.solana.orchestrator.domain.TransitionEvent
import com.kofiska.solana.orchestrator.ports.AuditPublisher

import scala.collection.concurrent.TrieMap
import scala.concurrent.{ExecutionContext, Future}

final class InMemoryAuditPublisher(implicit ec: ExecutionContext) extends AuditPublisher {
  private val events = TrieMap.empty[String, Vector[TransitionEvent]]

  override def publish(event: TransitionEvent): Future[Unit] =
    Future {
      val key = event.requestId
      val current = events.getOrElse(key, Vector.empty)
      events.put(key, current :+ event)
      ()
    }
}
