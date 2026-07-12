FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY *.go ./
RUN go test ./...
RUN go build -trimpath -o /out/multiwrite .

FROM alpine:3.22

WORKDIR /app
RUN mkdir -p \
    benchmarks/artifacts/outputs \
    benchmarks/artifacts/manifests \
    benchmarks/artifacts/profiles \
    benchmarks/artifacts/traces \
    benchmarks/artifacts/syscalls

COPY --from=build /out/multiwrite /usr/local/bin/multiwrite

ENTRYPOINT ["multiwrite"]
CMD ["run"]
