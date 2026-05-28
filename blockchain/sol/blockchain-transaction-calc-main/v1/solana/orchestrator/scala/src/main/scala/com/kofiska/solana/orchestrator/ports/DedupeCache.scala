package com.kofiska.solana.orchestrator.ports

import scala.concurrent.Future

trait DedupeCache {
  def get(requestId: String): Future[Option[String]]
  def claim(requestId: String, marker: String, ttlSeconds: Long): Future[Boolean]
  def put(requestId: String, decisionId: String, ttlSeconds: Long): Future[Unit]
  def delete(requestId: String): Future[Unit]
  def scan(prefix: String, limit: Int): Future[Vector[String]]
}
