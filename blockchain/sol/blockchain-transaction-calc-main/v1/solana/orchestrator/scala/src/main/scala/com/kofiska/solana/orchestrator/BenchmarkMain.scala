package com.kofiska.solana.orchestrator

import com.kofiska.solana.orchestrator.config.AppConfig
import com.kofiska.solana.orchestrator.infra.grpc.GrpcComputeGateway
import com.kofiska.solana.orchestrator.infra.postgres.JdbcDecisionRepository
import com.kofiska.solana.orchestrator.infra.redpanda.KafkaAuditPublisher
import com.kofiska.solana.orchestrator.infra.valkey.ValkeyDedupeCache
import com.kofiska.solana.orchestrator.domain._
import com.kofiska.solana.orchestrator.service.RequestWorkflow

import io.grpc.ManagedChannelBuilder

import java.nio.charset.StandardCharsets
import java.util.UUID
import scala.concurrent.duration._
import scala.concurrent.{Await, ExecutionContext, Future}
import scala.util.{Random, Try}

object BenchmarkMain {
  private final case class ScenarioResult(
    name: String,
    total: Int,
    terminalCounts: Map[String, Int],
    reasonCounts: Map[String, Int],
    evEstimate: Vector[BigDecimal],
    evLowerBound: Vector[BigDecimal],
    roundTripMs: Vector[Long]
  )

  def main(args: Array[String]): Unit = {
    implicit val ec: ExecutionContext = ExecutionContext.global
    val config = AppConfig.load()
    val channel = ManagedChannelBuilder.forAddress(config.computeHost, config.computePort).usePlaintext().build()
    val repo = new JdbcDecisionRepository(
      config.postgresUrl,
      config.postgresUser,
      config.postgresPassword,
      config.postgresPoolSize,
      config.postgresConnectionTimeoutMs
    )
    val cache = new ValkeyDedupeCache(config.valkeyUri)
    val audit = new KafkaAuditPublisher(config.auditBootstrapServers, config.auditTopic)
    val workflow = new RequestWorkflow(
      computeGateway = new GrpcComputeGateway(channel),
      decisionRepository = repo,
      dedupeCache = cache,
      auditPublisher = audit,
      dedupeTtlSeconds = config.dedupeTtlSeconds
    )

    try {
      val runId = UUID.randomUUID().toString
      val results = Seq(
        Await.result(burstMixed(workflow, runId), 10.minutes),
        Await.result(duplicateReplay(workflow, runId), 10.minutes),
        Await.result(freshAccept(workflow, runId), 10.minutes),
        Await.result(staleReject(workflow, runId), 10.minutes)
      )

      println("BENCHMARK_RESULTS_BEGIN")
      results.foreach(render)
      println("BENCHMARK_RESULTS_END")
    } finally {
      channel.shutdownNow()
      audit.close()
      cache.close()
      repo.close()
    }
  }

  private def burstMixed(workflow: RequestWorkflow, runId: String)(implicit ec: ExecutionContext): Future[ScenarioResult] = {
    val plan =
      Vector.fill(70)("accept") ++
        Vector.fill(20)("stale") ++
        Vector.fill(10)("oversized")
    val shuffled = Random.shuffle(plan)
    val futures = shuffled.zipWithIndex.map { case (kind, idx) =>
      timed {
        workflow.process(requestContext(runId, s"burst-$kind", idx, kind))
      }.map { case (result, elapsed) =>
        (result, elapsed)
      }
    }

    Future.sequence(futures).map { rows =>
      toResult("burst_100_mixed", rows)
    }
  }

  private def duplicateReplay(workflow: RequestWorkflow, runId: String)(implicit ec: ExecutionContext): Future[ScenarioResult] = {
    val ctx = requestContext(runId, "duplicate", 0, "accept")
    val first = timed(workflow.process(ctx))
    val second = first.flatMap { case (_, _) => timed(workflow.process(ctx)) }

    Future.sequence(Seq(first, second)).map { rows =>
      toResult("duplicate_replay", rows)
    }
  }

  private def freshAccept(workflow: RequestWorkflow, runId: String)(implicit ec: ExecutionContext): Future[ScenarioResult] = {
    val ctx = requestContext(runId, "single", 1, "accept")
    timed(workflow.process(ctx)).map { case row =>
      toResult("single_accept", Seq(row))
    }
  }

  private def staleReject(workflow: RequestWorkflow, runId: String)(implicit ec: ExecutionContext): Future[ScenarioResult] = {
    val ctx = requestContext(runId, "single", 2, "stale")
    timed(workflow.process(ctx)).map { case row =>
      toResult("single_stale", Seq(row))
    }
  }

  private def requestContext(runId: String, prefix: String, index: Int, kind: String): RequestContext = {
    val requestId = s"$prefix-request-$index-$runId"
    val routeCandidates = kind match {
      case "oversized" =>
        Vector.tabulate(17)(j => routeCandidate(s"$prefix-route-$index-$j-$runId", j))
      case _ =>
        Vector(
          routeCandidate(s"$prefix-route-$index-0-$runId", 0),
          routeCandidate(s"$prefix-route-$index-1-$runId", 1),
          routeCandidate(s"$prefix-route-$index-2-$runId", 2)
        )
    }

    RequestContext(
      requestId = requestId,
      dedupeKey = s"$prefix-dedupe-$index-$runId",
      traceId = s"$prefix-trace-$index-$runId",
      modelVersion = "v1",
      tokenIn = "USDC",
      tokenOut = "SOL",
      amountIn = "100.0",
      routeId = routeCandidates.headOption.map(_.routeId),
      slot = 123456789L + index,
      quoteAge = if (kind == "stale") 99L else 2L,
      sourceHashes = Vector(s"hash-$prefix-$index-a", s"hash-$prefix-$index-b"),
      routeCandidates = routeCandidates
    )
  }

  private def routeCandidate(routeId: String, hopIndex: Int): RouteCandidateInput =
    RouteCandidateInput(
      routeId = routeId,
      venue = if (hopIndex % 2 == 0) "pool" else "aggregator",
      hopCount = (hopIndex % 3) + 1
    )

  private def timed[T](future: Future[T])(implicit ec: ExecutionContext): Future[(T, Long)] = {
    val started = System.nanoTime()
    future.map(value => (value, nanosToMillis(System.nanoTime() - started)))
  }

  private def nanosToMillis(value: Long): Long =
    value / 1000000L

  private def toResult(name: String, rows: Seq[(DecisionResult, Long)]): ScenarioResult = {
    val terminalCounts = rows.iterator.map(_._1.terminalState).map(TerminalState.asString).toVector.groupMapReduce(identity)(_ => 1)(_ + _)
    val reasonCounts = rows.iterator.map(_._1.reasonCode).toVector.groupMapReduce(identity)(_ => 1)(_ + _)
    val evEstimate = rows.iterator.flatMap(_._1.evEstimate).toVector
    val evLowerBound = rows.iterator.flatMap(_._1.evLowerBound).toVector
    val roundTripMs = rows.iterator.map(_._2).toVector
    ScenarioResult(
      name = name,
      total = rows.size,
      terminalCounts = terminalCounts,
      reasonCounts = reasonCounts,
      evEstimate = evEstimate,
      evLowerBound = evLowerBound,
      roundTripMs = roundTripMs
    )
  }

  private def render(result: ScenarioResult): Unit = {
    val sorted = result.roundTripMs.sorted
    val p50 = percentile(sorted, 0.50)
    val p95 = percentile(sorted, 0.95)
    val p99 = percentile(sorted, 0.99)
    val evEstimate = stats(result.evEstimate)
    val evLowerBound = stats(result.evLowerBound)
    println(
      s"""SCENARIO ${result.name}
         | total=${result.total}
         | terminal=${formatMap(result.terminalCounts)}
         | reasons=${formatMap(result.reasonCounts)}
         | p50_ms=$p50
         | p95_ms=$p95
         | p99_ms=$p99
         | ev_estimate=${formatStats(evEstimate)}
         | ev_lower_bound=${formatStats(evLowerBound)}
         |""".stripMargin.trim
    )
  }

  private def percentile(values: Vector[Long], p: Double): Long = {
    if (values.isEmpty) 0L
    else {
      val idx = math.min(values.length - 1, math.ceil(values.length * p).toInt - 1)
      values(idx)
    }
  }

  private def formatMap(values: Map[String, Int]): String =
    values.toSeq.sortBy(_._1).map { case (k, v) => s"$k=$v" }.mkString(",")

  private def stats(values: Vector[BigDecimal]): (BigDecimal, BigDecimal, BigDecimal, BigDecimal) = {
    if (values.isEmpty) (BigDecimal(0), BigDecimal(0), BigDecimal(0), BigDecimal(0))
    else {
      val sorted = values.sorted
      val sum = sorted.sum
      val avg = sum / BigDecimal(sorted.size)
      (avg, sorted.head, sorted.last, sorted(sorted.size / 2))
    }
  }

  private def formatStats(values: (BigDecimal, BigDecimal, BigDecimal, BigDecimal)): String = {
    val (avg, min, max, median) = values
    s"avg=${avg.bigDecimal.toPlainString},min=${min.bigDecimal.toPlainString},median=${median.bigDecimal.toPlainString},max=${max.bigDecimal.toPlainString}"
  }
}
