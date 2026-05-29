package com.kofiska.solana.orchestrator

import akka.actor.testkit.typed.scaladsl.ScalaTestWithActorTestKit
import com.kofiska.solana.orchestrator.actors.IngressActor
import com.kofiska.solana.orchestrator.domain._
import com.kofiska.solana.orchestrator.ports.{AuditPublisher, ComputeGateway, DecisionRepository, DedupeCache}
import com.kofiska.solana.orchestrator.service.RequestWorkflow
import com.kofiska.solana.v1.decision.{Actionability => ProtoActionability, EvaluateSwapResponse, TerminalState => ProtoTerminalState}
import org.scalatest.wordspec.AnyWordSpecLike

import java.util.concurrent.ConcurrentHashMap
import scala.collection.concurrent.TrieMap
import scala.concurrent.ExecutionContext
import scala.concurrent.Future

final class IngressActorSpec extends ScalaTestWithActorTestKit with AnyWordSpecLike {
  implicit val ec: ExecutionContext = ExecutionContext.global

  "IngressActor" should {
    "forward a request through the workflow and return a terminal decision" in {
      val repository = new InMemoryDecisionRepository
      val cache = new InMemoryDedupeCache
      val audit = new RecordingAuditPublisher
      val workflow = new RequestWorkflow(new AcceptGateway, repository, cache, audit, dedupeTtlSeconds = 3600)
      val ingress = spawn(IngressActor(workflow))
      val replyTo = createTestProbe[DecisionResult]()

      ingress ! IngressActor.Submit(requestContext(), replyTo.ref)

      val result = replyTo.receiveMessage()
      result.terminalState shouldBe TerminalState.Accept
      result.actionability shouldBe Actionability.Actionable
      repository.state.get("req-1").terminalState shouldBe TerminalState.Accept
      audit.events.head.stage shouldBe "terminal"
    }
  }

  private def requestContext(): RequestContext =
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
      routeCandidates = Vector(
        RouteCandidateInput("route-a", "direct", 1),
        RouteCandidateInput("route-b", "aggregator", 4)
      )
    )

  private final class AcceptGateway extends ComputeGateway {
    override def evaluate(request: RequestContext): Future[EvaluateSwapResponse] =
      Future.successful(
        EvaluateSwapResponse(
          requestId = request.requestId,
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
      )
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

    override def delete(requestId: String): Future[Unit] =
      Future.successful {
        state.remove(requestId)
        ()
      }

    override def scan(prefix: String, limit: Int): Future[Vector[String]] =
      Future.successful {
        state.keySet().toArray.toVector.collect {
          case value: String if value.startsWith(prefix) => value
        }.take(limit)
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
