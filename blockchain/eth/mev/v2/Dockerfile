FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal

RUN go build -o /out/relay ./cmd/relay

FROM alpine:3.20

WORKDIR /app

COPY --from=build /out/relay /app/relay

EXPOSE 8090

ENTRYPOINT ["/app/relay"]
