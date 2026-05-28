scalaVersion := "2.13.12"

name := "solana-orchestrator"

Compile / mainClass := Some("com.kofiska.solana.orchestrator.OrchestratorMain")

Compile / PB.protoSources += baseDirectory.value / "../../../contracts"
Compile / PB.targets := Seq(scalapb.gen() -> (Compile / sourceManaged).value)

libraryDependencies ++= Seq(
  "com.typesafe.akka" %% "akka-actor-typed" % "2.8.5",
  "com.typesafe.akka" %% "akka-actor-testkit-typed" % "2.8.5" % Test,
  "com.typesafe.akka" %% "akka-stream" % "2.8.5",
  "com.typesafe.akka" %% "akka-slf4j" % "2.8.5",
  "com.thesamet.scalapb" %% "scalapb-runtime" % "0.11.15",
  "com.thesamet.scalapb" %% "scalapb-runtime-grpc" % "0.11.15",
  "org.postgresql" % "postgresql" % "42.7.4",
  "io.lettuce" % "lettuce-core" % "6.4.0.RELEASE",
  "org.apache.kafka" % "kafka-clients" % "3.8.1",
  "org.scalatest" %% "scalatest" % "3.2.18" % Test
)
