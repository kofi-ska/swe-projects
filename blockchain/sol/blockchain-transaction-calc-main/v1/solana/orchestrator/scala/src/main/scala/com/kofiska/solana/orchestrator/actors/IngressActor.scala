package com.kofiska.solana.orchestrator.actors

import akka.actor.typed.{ActorRef, Behavior}
import akka.actor.typed.scaladsl.Behaviors
import com.kofiska.solana.orchestrator.domain.{DecisionResult, RequestContext}

object IngressActor {
  sealed trait Command
  final case class Submit(ctx: RequestContext, replyTo: ActorRef[DecisionResult]) extends Command

  def apply(): Behavior[Command] =
    Behaviors.setup { context =>
      Behaviors.receiveMessage {
        case Submit(ctx, replyTo) =>
          context.log.info("ingress request {}", ctx.requestId)
          replyTo ! DecisionResult(
            requestId = ctx.requestId,
            decisionId = java.util.UUID.randomUUID().toString,
            terminalState = com.kofiska.solana.orchestrator.domain.TerminalState.Defer,
            reasonCode = "INGRESS_SCAFFOLD",
            actionability = "NON_ACTIONABLE",
            bestRouteId = ctx.routeId,
            evEstimate = None,
            evLowerBound = None
          )
          Behaviors.same
      }
    }
}

