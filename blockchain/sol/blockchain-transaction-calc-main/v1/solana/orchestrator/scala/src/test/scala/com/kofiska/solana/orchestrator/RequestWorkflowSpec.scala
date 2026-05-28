package com.kofiska.solana.orchestrator

import com.kofiska.solana.orchestrator.domain._
import com.kofiska.solana.orchestrator.ports.{AuditPublisher, ComputeGateway, DecisionRepository, DedupeCache}
import com.kofiska.solana.orchestrator.service.RequestWorkflow
import com.kofiska.solana.v1.{Actionability => ProtoActionability, EvaluateSwapResponse, TerminalState => ProtoTerminalState}
import org.scalatest.flatspec.AsyncFlatSpec
import org.scalatest.matchers.should.Matchers

import java.util.concurrent.ConcurrentHashMap
import scala.collection.concurrent.TrieMap
import scala.concurrent.{ExecutionContext, Future}

final class RequestWorkflowSpec extends AsyncFlatSpec with Matchers {
  implicit override def executionContext: ExecutionContext = ExecutionContext.global

  private def context(routeCandidates: Vector[RouteCandidateInput] = defaultRoutes): RequestContext =
    RequestContext(
      requestId = "req-1",
      dedupeKey = "dedupe-1",
      traceId = "trace-1",
      modelVersion = "v1",
      tokenIn = "USDC",
      tokenOut = "SOL",
      amountIn = "1000",
      routeId = Some("route-a"),
      slot = 100,
      quoteAge = 1,
      sourceHashes = Vector("hash-a", "hash-b"),
      routeCandidates = routeCandidates
    )

  private val defaultRoutes = Vector(
    RouteCandidateInput("route-a", "direct", 1),
    RouteCandidateInput("route-b", "aggregator", 4)
  )

  private def acceptResponse(requestId: String = "req-1"): EvaluateSwapResponse =
    EvaluateSwapResponse(
      requestId = requestId,
      decisionId = "decision-1",
      terminalState = ProtoTerminalState.ACCEPT,
      actionability = ProtoActionability.ACTIONABLE,
      reasonCode = "ACCEPTED",
      bestRouteId = "route-a",
      expectedOutput = "1016.200000",
      feeCost = "0.780000",
      slippageCost = "1.070000",
      breakevenMargin = "14.350000",
      evEstimate = "13.700000",
      evLowerBound = "13.200000",
      riskScore = "0.001700",
      freshnessValid = true,
      computeLatencyMs = 7L,
      sourceHashes = Vector("hash-a", "hash-b")
    )

  it should "persist accept decisions and emit audit events" in {
    val repository = new InMemoryDecisionRepository
    val cache = new InMemoryDedupeCache
    val audit = new RecordingAuditPublisher
    val workflow = new RequestWorkflow(new AcceptGateway, repository, cache, audit, dedupeTtlSeconds = 3600)

    workflow.process(context()).map { result =>
      result.terminalState shouldBe TerminalState.Accept
      result.actionability shouldBe Actionability.Actionable
      result.decisionId shouldBe "decision-1"
      Option(repository.state.get("req-1")).get.terminalState shouldBe TerminalState.Accept
      Option(cache.state.get("req-1")).get shouldBe "decision-1"
      audit.events.size shouldBe 1
      audit.events.head.stage shouldBe "terminal"
      audit.events.head.success shouldBe true
      succeed
    }
  }

  it should "replay durable decisions without recomputing" in {
    val repository = new InMemoryDecisionRepository
    val cache = new InMemoryDedupeCache
    val audit = new RecordingAuditPublisher
    val workflow = new RequestWorkflow(new FailingGateway, repository, cache, audit, dedupeTtlSeconds = 3600)

    val cached = DecisionResult(
      requestId = "req-1",
      decisionId = "decision-1",
      terminalState = TerminalState.Accept,
      reasonCode = "ACCEPTED",
      actionability = Actionability.Actionable,
      bestRouteId = Some("route-a"),
      sourceHashes = Vector("hash-a", "hash-b"),
      expectedOutput = Some(BigDecimal("1016.200000")),
      feeCost = Some(BigDecimal("0.780000")),
      slippageCost = Some(BigDecimal("1.070000")),
      breakevenMargin = Some(BigDecimal("14.350000")),
      evEstimate = Some(BigDecimal("13.700000")),
      evLowerBound = Some(BigDecimal("13.200000")),
      riskScore = Some(BigDecimal("0.001700")),
      freshnessValid = true
    )

    repository.state.put("req-1", cached)
    cache.state.put("req-1", "decision-1")

    workflow.process(context()).map { result =>
      result shouldBe cached
      audit.events.size shouldBe 1
      audit.events.head.stage shouldBe "replay"
      audit.events.head.reasonCode shouldBe "REPLAY_HIT"
      succeed
    }
  }

  it should "return a duplicate-in-flight failure when the terminal row is missing" in {
    val repository = new InMemoryDecisionRepository
    val cache = new InMemoryDedupeCache
    val audit = new RecordingAuditPublisher
    val workflow = new RequestWorkflow(new FailingGateway, repository, cache, audit, dedupeTtlSeconds = 3600)

    cache.state.put("req-1", "decision-1")

    workflow.process(context()).map { result =>
      result.terminalState shouldBe TerminalState.Failed
      result.reasonCode shouldBe "DUPLICATE_INFLIGHT_REQUEST"
      repository.state.get("req-1") shouldBe null
      audit.events shouldBe empty
      succeed
    }
  }

  private final class AcceptGateway extends ComputeGateway {
    override def evaluate(request: RequestContext): Future[EvaluateSwapResponse] =
      Future.successful(acceptResponse(request.requestId))
  }

  private final class FailingGateway extends ComputeGateway {
    override def evaluate(request: RequestContext): Future[EvaluateSwapResponse] =
      Future.failed(new IllegalStateException("compute should not run"))
  }

  private final class InMemoryDecisionRepository extends DecisionRepository {
    val state = new ConcurrentHashMap[String, DecisionResult]()
    val audit = TrieMap.empty[(String, String), TransitionEvent]
    val published = TrieMap.empty[(String, String), Boolean]

    override def find(requestId: String): Future[Option[DecisionResult]] =
      Future.successful(Option(state.get(requestId)))

    override def upsert(ctx: RequestContext, result: DecisionResult, event: TransitionEvent): Future[Unit] =
      Future.successful {
        state.putIfAbsent(ctx.requestId, result)
        audit.putIfAbsent((ctx.requestId, result.decisionId), event)
        ()
      }

    override def pendingAudit(limit: Int): Future[Vector[TransitionEvent]] =
      Future.successful(audit.collect { case (key, event) if !published.contains(key) => event }.take(limit).toVector)

    override def markAuditPublished(requestId: String, decisionId: String): Future[Unit] =
      Future.successful {
        published.put((requestId, decisionId), true)
        ()
      }
  }

  private final class InMemoryDedupeCache extends DedupeCache {
    val state = new ConcurrentHashMap[String, String]()

    override def get(requestId: String): Future[Option[String]] =
      Future.successful(Option(state.get(requestId)))

    override def claim(requestId: String, marker: String, ttlSeconds: Long): Future[Boolean] =
      Future.successful(state.putIfAbsent(requestId, marker) == null)

    override def put(requestId: String, decisionId: String, ttlSeconds: Long): Future[Unit] =
      Future.successful {
        state.put(requestId, decisionId)
        ()
      }
  }

  private final class RecordingAuditPublisher extends AuditPublisher {
    val events = scala.collection.mutable.ArrayBuffer.empty[TransitionEvent]

    override def publish(event: TransitionEvent): Future[Unit] =
      Future.successful {
        events += event
        ()
      }
  }
}
