FROM rust:1.82-slim AS builder

WORKDIR /workspace/v1

COPY contracts ./contracts
COPY solana/compute/rust ./solana/compute/rust

WORKDIR /workspace/v1/solana/compute/rust

RUN cargo build --release

FROM debian:bookworm-slim

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /workspace/v1/solana/compute/rust/target/release/solana-compute /usr/local/bin/solana-compute

ENV COMPUTE_ADDR=0.0.0.0:50051

EXPOSE 50051

ENTRYPOINT ["solana-compute"]
