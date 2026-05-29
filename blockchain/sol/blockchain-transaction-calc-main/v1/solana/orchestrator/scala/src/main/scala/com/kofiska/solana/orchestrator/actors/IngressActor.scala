package com.kofiska.solana.orchestrator.actors

import akka.actor.typed.{ActorRef, Behavior}
import akka.actor.typed.scaladsl.Behaviors
import com.kofiska.solana.orchestrator.domain.{DecisionResult, RequestContext}
import com.kofiska.solana.orchestrator.service.RequestWorkflow

object IngressActor {
  sealed trait Command
  final case class Submit(ctx: RequestContext, replyTo: ActorRef[DecisionResult]) extends Command

  def apply(workflow: RequestWorkflow): Behavior[Command] =
    Behaviors.setup { context =>
      Behaviors.receiveMessage {
        case Submit(ctx, replyTo) =>
          context.log.info("ingress request {}", ctx.requestId)
          val requestActor = context.spawnAnonymous(RequestActor(workflow))
          requestActor ! RequestActor.Start(ctx, replyTo)
          Behaviors.same
      }
    }
}
