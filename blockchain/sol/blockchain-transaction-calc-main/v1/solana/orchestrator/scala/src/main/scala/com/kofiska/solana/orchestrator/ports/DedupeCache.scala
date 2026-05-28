package com.kofiska.solana.orchestrator.ports

import scala.concurrent.Future

trait DedupeCache {
  def get(requestId: String): Future[Option[String]]
  def put(requestId: String, decisionId: String, ttlSeconds: Long): Future[Unit]
}

