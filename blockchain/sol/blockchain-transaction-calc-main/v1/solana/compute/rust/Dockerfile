FROM rust:1.82-slim@sha256:1111c28d995d06a7863ba6cea3b3dcb87bebe65af8ec5517caaf2c8c26f38010 AS builder

WORKDIR /workspace/v1

COPY contracts ./contracts
COPY solana/compute/rust ./solana/compute/rust

WORKDIR /workspace/v1/solana/compute/rust

RUN cargo build --release

FROM debian:bookworm-slim@sha256:0104b334637a5f19aa9c983a91b54c89887c0984081f2068983107a6f6c21eeb

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates bash \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /workspace/v1/solana/compute/rust/target/release/solana-compute /usr/local/bin/solana-compute

ENV COMPUTE_ADDR=0.0.0.0:50051

EXPOSE 50051

ENTRYPOINT ["solana-compute"]
