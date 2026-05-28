package com.kofiska.solana.orchestrator.actors

import akka.actor.typed.Behavior
import akka.actor.typed.scaladsl.Behaviors
import com.kofiska.solana.orchestrator.domain.RequestContext

object PolicyActor {
  sealed trait Command
  final case class Evaluate(ctx: RequestContext) extends Command

  def apply(): Behavior[Command] =
    Behaviors.receiveMessage {
      case Evaluate(ctx) =>
        Behaviors.same
    }
}

