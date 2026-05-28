package com.kofiska.solana.orchestrator.ports

import com.kofiska.solana.orchestrator.domain.TransitionEvent

import scala.concurrent.Future

trait AuditPublisher {
  def publish(event: TransitionEvent): Future[Unit]
}

