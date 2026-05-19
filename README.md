# Policy Decision Service

Policy Decision Service (PDS) evaluates publication and moderation requests with deterministic rules over HTTP and gRPC. It can enrich decisions with VideoProcess actor features, write audit records, publish Kafka decision events, and reload rules without restarting the process.

## Quickstart

```bash
cp config/server.example.env .env
go run ./cmd/server
```

By default the server listens on `:8080` for HTTP and `:9090` for gRPC, loads rules from `config/rules.example.yaml`, and exposes `/healthz`, `/readyz`, and `/metrics`.

## HTTP

Send decisions to `POST /v1/decide` with `X-Client-Id` set:

```bash
curl -sS -X POST http://localhost:8080/v1/decide \
  -H 'Content-Type: application/json' \
  -H 'X-Client-Id: local' \
  -d '{
    "actor_id": "actor-123",
    "action": {"type": "publish", "platform": "youtube"},
    "content": {"title": "demo", "duration_s": 30, "tags": ["test"]},
    "context": {"request_source": "curl"}
  }'
```

## gRPC

The protobuf contract is in `proto/pds/v1/pds.proto`; generated Go code lives under `proto/gen/pds/v1`. Regenerate it with:

```bash
go install github.com/bufbuild/buf/cmd/buf@latest
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
PATH="$(go env GOPATH)/bin:$PATH" buf generate
```

The `PolicyDecisionService.Decide` RPC maps to the same engine and response fields as HTTP. gRPC requests use client id `grpc` for metrics and audit records.

## Rules And Reload

Rules are loaded from `PDS_RULES_PATH`, defaulting to `config/rules.example.yaml`. Reload rules without restarting:

```bash
curl -sS -X POST http://localhost:8080/v1/admin/reload
```

The process also reloads rules on `SIGHUP`.

## Feature Provider

Set `PDS_FEATURE_PROVIDER_URL` to enable actor feature enrichment, for example `http://vp-feature-aggregator:8080`. Feature-backed CEL rules can read fields such as `features.publishes_5m` and `degraded.feature_provider`.

## Kafka Sink

Enable decision event publishing with:

```bash
PDS_KAFKA_ENABLED=true
PDS_KAFKA_BROKERS=redpanda:9092
PDS_KAFKA_DECISION_TOPIC=pds.decisions.v1
```

Kafka sinks are stopped after HTTP and gRPC shutdown so queued decisions can drain before publisher resources close.

## Kubernetes

Apply the sample manifest for the `videoprocess` namespace:

```bash
kubectl apply -f deploy/kubernetes.yaml
```

It creates `pds-config`, a single-replica `pds` Deployment, and a Service exposing `8080/http` and `9090/grpc`.

## Tests

```bash
gofmt -w cmd internal proto
go test ./... -count=1
go build ./cmd/server
```
