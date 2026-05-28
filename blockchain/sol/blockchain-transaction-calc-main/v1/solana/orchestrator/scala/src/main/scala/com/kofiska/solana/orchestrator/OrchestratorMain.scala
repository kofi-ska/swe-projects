package com.kofiska.solana.orchestrator

import akka.actor.typed.ActorSystem
import com.kofiska.solana.orchestrator.actors.IngressActor

object OrchestratorMain {
  def main(args: Array[String]): Unit = {
    ActorSystem(IngressActor(), "solana-orchestrator")
  }
}
