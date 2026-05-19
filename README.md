# Policy Decision Service

Policy Decision Service (PDS) evaluates publication and moderation requests with deterministic rules over HTTP and gRPC. It can enrich decisions with VideoProcess actor features, write audit records, publish Kafka decision events, and reload rules without restarting the process.

## Quickstart

```bash
go run ./cmd/server
```

By default the server listens on `:8080` for HTTP and `:9090` for gRPC, loads rules from `config/rules.example.yaml`, and exposes `/healthz`, `/readyz`, and `/metrics`.
The process reads environment variables directly; it does not automatically load `.env` files. To run with `config/server.example.env`, source it in the shell and override the rule paths for a repo-local run:

```bash
set -a
source config/server.example.env
set +a
PDS_RULES_PATH=config/rules.example.yaml go run ./cmd/server
```

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
Request `context` and response `metadata` use `google.protobuf.Struct` so callers can preserve JSON-like nested objects, arrays, booleans, numbers, and strings.

## Rules And Reload

Rules are loaded from `PDS_RULES_PATH`, defaulting to `config/rules.example.yaml`. Keyword rule files are declared per rule with `keywords_file` and are resolved relative to the rule file unless an absolute path is used. Reload rules without restarting:

```bash
curl -sS -X POST http://localhost:8080/v1/admin/reload
```

The process also reloads rules on `SIGHUP`.

## Feature Provider

Set `PDS_FEATURE_PROVIDER_URL` to enable actor feature enrichment, for example `http://vp-feature-aggregator:8080`. PDS reads `GET /v1/features/{actor_id}` and maps fields including `features.publishes_5m`, `features.publishes_1h`, `features.publishes_24h`, `features.blocks_24h`, `features.flags_7d`, and `features.comment_burst_1m`. Feature-backed CEL rules can also check `degraded.feature_provider`; when the provider is unavailable, decisions fail open with zero-value features and a `feature_provider_unavailable` warning.

## Kafka Sink

Enable decision event publishing with:

```bash
PDS_KAFKA_ENABLED=true
PDS_KAFKA_BROKERS=redpanda:9092
PDS_KAFKA_DECISION_TOPIC=pds.decisions.v1
PDS_KAFKA_CLIENT_ID=pds
PDS_KAFKA_QUEUE_SIZE=10000
```

Kafka publishing is asynchronous and should not be treated as the durable audit store. Durable audit writes use Postgres when `PDS_DATABASE_URL` is reachable; Kafka publish errors and queue drops are exposed through metrics. Kafka sinks are stopped after HTTP and gRPC shutdown so queued decisions can drain before publisher resources close.

## VideoProcess Compose Integration

The VP repo owns the local compose wiring in `docker-compose.pds-kafka.yml`. From the VP worktree, run it with the base compose file:

```bash
docker compose -f docker-compose.yml -f docker-compose.pds-kafka.yml config
docker compose -f docker-compose.yml -f docker-compose.pds-kafka.yml up -d --build redpanda pds vp-feature-aggregator event-outbox-relay
```

## Kubernetes

Apply the sample manifest for the `videoprocess` namespace:

```bash
kubectl apply -f deploy/kubernetes.yaml
```

It creates sample `pds-config` and `pds-rules` ConfigMaps, a single-replica `pds` Deployment, and a Service exposing `8080/http` and `9090/grpc`. The manifest is local/sample wiring: adjust the image, database URL, Redis URL, Kafka brokers, feature-provider URL, and rule content for the target cluster, and move credentials such as `PDS_DATABASE_URL` into a Secret before production use.

## Tests

```bash
PATH="$(go env GOPATH)/bin:$PATH" buf generate
buf lint
gofmt -w cmd internal proto
go test ./... -count=1
go build ./cmd/server
git diff --check
```
