package com.kofiska.solana.orchestrator

import com.kofiska.solana.orchestrator.domain._
import com.kofiska.solana.orchestrator.ports.{AuditPublisher, ComputeGateway, DecisionRepository, DedupeCache}
import com.kofiska.solana.orchestrator.service.RequestWorkflow
import com.kofiska.solana.v1.{Actionability => ProtoActionability, EvaluateSwapResponse, TerminalState => ProtoTerminalState}
import org.scalatest.flatspec.AsyncFlatSpec
import org.scalatest.matchers.should.Matchers

import java.util.concurrent.ConcurrentHashMap
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
    val workflow = new RequestWorkflow(new AcceptGateway, repository, cache, audit)

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
    val workflow = new RequestWorkflow(new FailingGateway, repository, cache, audit)

    val cached = DecisionResult(
      requestId = "req-1",
      decisionId = "decision-1",
      terminalState = TerminalState.Accept,
      reasonCode = "ACCEPTED",
      actionability = Actionability.Actionable,
      bestRouteId = Some("route-a"),
      expectedOutput = Some("1016.200000"),
      feeCost = Some("0.780000"),
      slippageCost = Some("1.070000"),
      breakevenMargin = Some("14.350000"),
      evEstimate = Some("13.700000"),
      evLowerBound = Some("13.200000"),
      riskScore = Some("0.001700"),
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

  it should "fail closed when dedupe exists but the terminal row is missing" in {
    val repository = new InMemoryDecisionRepository
    val cache = new InMemoryDedupeCache
    val audit = new RecordingAuditPublisher
    val workflow = new RequestWorkflow(new FailingGateway, repository, cache, audit)

    cache.state.put("req-1", "decision-1")

    workflow.process(context()).map { result =>
      result.terminalState shouldBe TerminalState.Failed
      result.reasonCode shouldBe "DEDUPED_BUT_MISSING_TERMINAL_RECORD"
      Option(repository.state.get("req-1")).get.terminalState shouldBe TerminalState.Failed
      audit.events.head.stage shouldBe "terminal"
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

    override def find(requestId: String): Future[Option[DecisionResult]] =
      Future.successful(Option(state.get(requestId)))

    override def upsert(ctx: RequestContext, result: DecisionResult): Future[Unit] =
      Future.successful {
        state.put(ctx.requestId, result)
        ()
      }
  }

  private final class InMemoryDedupeCache extends DedupeCache {
    val state = new ConcurrentHashMap[String, String]()

    override def get(requestId: String): Future[Option[String]] =
      Future.successful(Option(state.get(requestId)))

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
