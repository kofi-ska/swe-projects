package com.kofiska.solana.orchestrator.actors

import akka.actor.typed.{ActorRef, Behavior}
import akka.actor.typed.scaladsl.Behaviors
import com.kofiska.solana.orchestrator.domain.{DecisionResult, RequestContext}
import com.kofiska.solana.orchestrator.service.RequestWorkflow

import scala.util.{Failure, Success}

object RequestActor {
  sealed trait Command
  final case class Start(ctx: RequestContext, replyTo: ActorRef[DecisionResult]) extends Command
  private final case class WorkflowCompleted(result: DecisionResult, replyTo: ActorRef[DecisionResult]) extends Command
  private final case class WorkflowFailed(reason: String, replyTo: ActorRef[DecisionResult]) extends Command

  def apply(workflow: RequestWorkflow): Behavior[Command] =
    Behaviors.setup { context =>
      Behaviors.receiveMessage {
        case Start(ctx, replyTo) =>
          context.log.info("start request {}", ctx.requestId)
          context.pipeToSelf(workflow.process(ctx)) {
            case Success(result) => WorkflowCompleted(result, replyTo)
            case Failure(error)  => WorkflowFailed(error.getMessage, replyTo)
          }
          Behaviors.same

        case WorkflowCompleted(result, replyTo) =>
          replyTo ! result
          Behaviors.stopped

        case WorkflowFailed(reason, replyTo) =>
          replyTo ! DecisionResult(
            requestId = "",
            decisionId = java.util.UUID.randomUUID().toString,
            terminalState = com.kofiska.solana.orchestrator.domain.TerminalState.Failed,
            reasonCode = reason,
            actionability = "NON_ACTIONABLE",
            bestRouteId = None,
            evEstimate = None,
            evLowerBound = None
          )
          Behaviors.stopped
      }
    }
}
