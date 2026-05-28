package com.kofiska.solana.orchestrator.config

final case class AppConfig(
  computeHost: String,
  computePort: Int,
  postgresUrl: String,
  postgresUser: String,
  postgresPassword: String,
  valkeyUri: String,
  auditBootstrapServers: String,
  auditTopic: String
)

object AppConfig {
  def load(): AppConfig =
    AppConfig(
      computeHost = env("COMPUTE_HOST", "127.0.0.1"),
      computePort = env("COMPUTE_PORT", "50051").toInt,
      postgresUrl = env("POSTGRES_URL", "jdbc:postgresql://127.0.0.1:5432/decision_store"),
      postgresUser = env("POSTGRES_USER", "decision"),
      postgresPassword = env("POSTGRES_PASSWORD", "decision"),
      valkeyUri = env("VALKEY_URI", "redis://127.0.0.1:6379/0"),
      auditBootstrapServers = env("AUDIT_BOOTSTRAP_SERVERS", "127.0.0.1:9092"),
      auditTopic = env("AUDIT_TOPIC", "solana.audit")
    )

  private def env(name: String, default: String): String =
    sys.env.getOrElse(name, default)
}
