package com.kofiska.solana.orchestrator.infra.valkey

import com.kofiska.solana.orchestrator.ports.DedupeCache
import io.lettuce.core.codec.StringCodec
import io.lettuce.core.{RedisClient, RedisURI}

import scala.concurrent.{ExecutionContext, Future, blocking}

final class ValkeyDedupeCache(redisUri: String)(implicit ec: ExecutionContext) extends DedupeCache {
  private val client = RedisClient.create(RedisURI.create(redisUri))
  private val connection = client.connect(StringCodec.UTF8)
  private val commands = connection.sync()

  override def get(requestId: String): Future[Option[String]] =
    Future(blocking {
      Option(commands.get(key(requestId)))
    })

  override def put(requestId: String, decisionId: String, ttlSeconds: Long): Future[Unit] =
    Future(blocking {
      commands.setex(key(requestId), ttlSeconds, decisionId)
      ()
    })

  private def key(requestId: String): String =
    s"solana:dedupe:$requestId"
}
