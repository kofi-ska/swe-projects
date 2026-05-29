package com.kofiska.solana.orchestrator.infra.redpanda

import com.kofiska.solana.orchestrator.domain.TransitionEvent
import com.kofiska.solana.orchestrator.ports.AuditPublisher
import org.apache.kafka.clients.producer.{KafkaProducer, ProducerRecord}

import java.util.Properties
import java.util.concurrent.TimeUnit
import scala.concurrent.{ExecutionContext, Future, blocking}

final class KafkaAuditPublisher(bootstrapServers: String, topic: String)(implicit ec: ExecutionContext)
    extends AuditPublisher {

  private val producer = new KafkaProducer[String, String](producerProperties(bootstrapServers))

  override def publish(event: TransitionEvent): Future[Unit] =
    Future(blocking {
      val record = new ProducerRecord[String, String](topic, event.requestId, toJson(event))
      producer.send(record).get(5, TimeUnit.SECONDS)
      ()
    })

  private def producerProperties(bootstrapServers: String): Properties = {
    val props = new Properties()
    props.put("bootstrap.servers", bootstrapServers)
    props.put("key.serializer", "org.apache.kafka.common.serialization.StringSerializer")
    props.put("value.serializer", "org.apache.kafka.common.serialization.StringSerializer")
    props.put("acks", "all")
    props.put("enable.idempotence", "true")
    props.put("delivery.timeout.ms", "120000")
    props.put("request.timeout.ms", "30000")
    props
  }

  private def toJson(event: TransitionEvent): String =
    {
      val hashes = event.sourceHashes.map(value => quote(value)).mkString(",")
      s"""{"schema_version":${quote(event.schemaVersion)},"trace_id":${quote(event.traceId)},"request_id":${quote(event.requestId)},"decision_id":${quote(event.decisionId)},"terminal_state":${quote(event.terminalState)},"reason_code":${quote(event.reasonCode)},"model_version":${quote(event.modelVersion)},"route_id":${stringOrNull(event.routeId)},"slot":${event.slot},"quote_age":${event.quoteAge},"source_hashes":[$hashes],"stage":${quote(event.stage)},"latency_ms":${event.latencyMs},"bytes_in":${event.bytesIn},"bytes_out":${event.bytesOut},"success":${event.success}}"""
    }

  private def stringOrNull(value: Option[String]): String =
    value.map(quote).getOrElse("null")

  private def quote(value: String): String =
    s""""${escape(value)}""""

  private def escape(value: String): String =
    value.replace("\\", "\\\\").replace("\"", "\\\"")

  def close(): Unit =
    producer.close()
}
