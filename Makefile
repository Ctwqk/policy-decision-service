.PHONY: test build run docker-build fmt

test:
	go test ./...

build:
	CGO_ENABLED=0 go build -o bin/pds ./cmd/server

run:
	go run ./cmd/server

docker-build:
	docker build -f deploy/Dockerfile -t policy-decision-service:local .

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './proto/gen/*')
