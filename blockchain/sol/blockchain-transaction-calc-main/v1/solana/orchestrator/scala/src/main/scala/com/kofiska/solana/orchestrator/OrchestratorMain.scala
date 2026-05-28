package com.kofiska.solana.orchestrator

import akka.actor.typed.ActorSystem
import com.kofiska.solana.orchestrator.actors.IngressActor
import com.kofiska.solana.orchestrator.config.AppConfig
import com.kofiska.solana.orchestrator.domain.RequestContext
import com.kofiska.solana.orchestrator.infra.grpc.GrpcComputeGateway
import com.kofiska.solana.orchestrator.infra.postgres.JdbcDecisionRepository
import com.kofiska.solana.orchestrator.infra.redpanda.KafkaAuditPublisher
import com.kofiska.solana.orchestrator.infra.valkey.ValkeyDedupeCache
import com.kofiska.solana.orchestrator.service.RequestWorkflow

import io.grpc.ManagedChannelBuilder

import scala.concurrent.ExecutionContext

object OrchestratorMain {
  def main(args: Array[String]): Unit = {
    implicit val ec: ExecutionContext = ExecutionContext.global
    val config = AppConfig.load()
    val channel = ManagedChannelBuilder
      .forAddress(config.computeHost, config.computePort)
      .usePlaintext()
      .build()

    val workflow = new RequestWorkflow(
      computeGateway = new GrpcComputeGateway(channel),
      decisionRepository = new JdbcDecisionRepository(config.postgresUrl, config.postgresUser, config.postgresPassword),
      dedupeCache = new ValkeyDedupeCache(config.valkeyUri),
      auditPublisher = new KafkaAuditPublisher(config.auditBootstrapServers, config.auditTopic)
    )

    val system = ActorSystem(IngressActor(workflow), "solana-orchestrator")
    system.log.info("orchestrator started")

    sys.addShutdownHook {
      channel.shutdownNow()
      system.terminate()
    }
  }
}
