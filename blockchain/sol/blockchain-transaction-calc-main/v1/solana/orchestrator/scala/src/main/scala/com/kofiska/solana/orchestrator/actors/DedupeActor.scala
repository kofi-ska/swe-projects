package com.kofiska.solana.orchestrator.actors

import akka.actor.typed.Behavior
import akka.actor.typed.scaladsl.Behaviors

object DedupeActor {
  sealed trait Command
  final case class Check(requestId: String, dedupeKey: String) extends Command

  def apply(): Behavior[Command] =
    Behaviors.receiveMessage {
      case Check(requestId, dedupeKey) =>
        Behaviors.same
    }
}

