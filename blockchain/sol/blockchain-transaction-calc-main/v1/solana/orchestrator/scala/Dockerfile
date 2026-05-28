FROM eclipse-temurin:17-jdk-jammy@sha256:beabb759e6f9653c843958d1d1f5cecb881dfb85aa6081e2bef099ab1260344e

ENV DEBIAN_FRONTEND=noninteractive

WORKDIR /workspace/v1

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
       ca-certificates \
       curl \
    && curl -fsSL https://github.com/sbt/sbt/releases/download/v1.10.7/sbt-1.10.7.tgz \
       | tar -xz -C /opt \
    && ln -s /opt/sbt/bin/sbt /usr/local/bin/sbt \
    && rm -rf /var/lib/apt/lists/*

COPY contracts ./contracts
COPY solana/orchestrator/scala ./solana/orchestrator/scala

WORKDIR /workspace/v1/solana/orchestrator/scala

ENV COMPUTE_HOST=compute \
    COMPUTE_PORT=50051 \
    POSTGRES_URL=jdbc:postgresql://postgres:5432/decision_store \
    POSTGRES_USER=decision \
    POSTGRES_PASSWORD=decision \
    VALKEY_URI=redis://valkey:6379/0 \
    AUDIT_BOOTSTRAP_SERVERS=redpanda:9092 \
    AUDIT_TOPIC=solana.audit

CMD ["sbt", "-Dsbt.color=false", "-batch", "runMain", "com.kofiska.solana.orchestrator.OrchestratorMain"]
