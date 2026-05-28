package com.kofiska.solana.orchestrator

import akka.actor.typed.ActorSystem
import com.kofiska.solana.orchestrator.actors.IngressActor
import com.kofiska.solana.orchestrator.config.AppConfig
import com.kofiska.solana.orchestrator.infra.grpc.GrpcComputeGateway
import com.kofiska.solana.orchestrator.infra.postgres.JdbcDecisionRepository
import com.kofiska.solana.orchestrator.infra.redpanda.KafkaAuditPublisher
import com.kofiska.solana.orchestrator.infra.valkey.ValkeyDedupeCache
import com.kofiska.solana.orchestrator.service.{ReconciliationService, RequestWorkflow}

import io.grpc.ManagedChannelBuilder

import scala.concurrent.ExecutionContext
import scala.concurrent.duration._

object OrchestratorMain {
  def main(args: Array[String]): Unit = {
    implicit val ec: ExecutionContext = ExecutionContext.global
    val config = AppConfig.load()
    val channel = ManagedChannelBuilder
      .forAddress(config.computeHost, config.computePort)
      .usePlaintext()
      .build()

    val decisionRepository = new JdbcDecisionRepository(
      config.postgresUrl,
      config.postgresUser,
      config.postgresPassword,
      config.postgresPoolSize,
      config.postgresConnectionTimeoutMs
    )
    val dedupeCache = new ValkeyDedupeCache(config.valkeyUri)
    val auditPublisher = new KafkaAuditPublisher(config.auditBootstrapServers, config.auditTopic)
    val workflow = new RequestWorkflow(
      computeGateway = new GrpcComputeGateway(channel),
      decisionRepository = decisionRepository,
      dedupeCache = dedupeCache,
      auditPublisher = auditPublisher,
      dedupeTtlSeconds = config.dedupeTtlSeconds
    )
    val reconciliationService = new ReconciliationService(
      decisionRepository = decisionRepository,
      dedupeCache = dedupeCache,
      auditPublisher = auditPublisher,
      dedupeTtlSeconds = config.dedupeTtlSeconds
    )

    val system = ActorSystem(IngressActor(workflow), "solana-orchestrator")
    system.log.info("orchestrator started")
    reconciliationService.runOnce(100)
    val reconciliationTask = system.scheduler.scheduleAtFixedRate(5.minutes, 5.minutes)(
      new Runnable {
        override def run(): Unit =
          reconciliationService.runOnce(100)
      }
    )(system.executionContext)

    sys.addShutdownHook {
      reconciliationTask.cancel()
      channel.shutdownNow()
      auditPublisher.close()
      dedupeCache.close()
      decisionRepository.close()
      system.terminate()
    }
  }
}
