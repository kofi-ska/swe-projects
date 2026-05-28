package com.kofiska.solana.orchestrator.actors

import akka.actor.typed.{ActorRef, Behavior}
import akka.actor.typed.scaladsl.Behaviors
import com.kofiska.solana.orchestrator.domain.{Actionability, DecisionResult, RequestContext, TerminalState}
import com.kofiska.solana.orchestrator.service.RequestWorkflow

import scala.util.{Failure, Success}

object RequestActor {
  sealed trait Command
  final case class Start(ctx: RequestContext, replyTo: ActorRef[DecisionResult]) extends Command
  private final case class WorkflowCompleted(result: DecisionResult, replyTo: ActorRef[DecisionResult]) extends Command
  private final case class WorkflowFailed(ctx: RequestContext, reason: String, replyTo: ActorRef[DecisionResult]) extends Command

  def apply(workflow: RequestWorkflow): Behavior[Command] =
    Behaviors.setup { context =>
      Behaviors.receiveMessage {
        case Start(ctx, replyTo) =>
          context.log.info("request {} started", ctx.requestId)
          context.pipeToSelf(workflow.process(ctx)) {
            case Success(result) => WorkflowCompleted(result, replyTo)
            case Failure(error)  => WorkflowFailed(ctx, error.getMessage, replyTo)
          }
          Behaviors.same

        case WorkflowCompleted(result, replyTo) =>
          replyTo ! result
          Behaviors.stopped

        case WorkflowFailed(ctx, reason, replyTo) =>
          replyTo ! DecisionResult(
            requestId = ctx.requestId,
            decisionId = java.util.UUID.randomUUID().toString,
            terminalState = TerminalState.Failed,
            reasonCode = if (reason.nonEmpty) reason else "WORKFLOW_FAILED",
            actionability = Actionability.NonActionable,
            bestRouteId = ctx.routeId,
            expectedOutput = None,
            feeCost = None,
            slippageCost = None,
            breakevenMargin = None,
            evEstimate = None,
            evLowerBound = None,
            riskScore = None,
            freshnessValid = false
          )
          Behaviors.stopped
      }
    }
}
