.PHONY: all build example images sandbox-smoke smoke check lint

all: build example

build:
	go build -trimpath -o bin/giad ./cmd/giad

example:
	go build -trimpath -o bin/diff-inspector ./example/diff-inspector
	./bin/diff-inspector --manifest --container > bin/diff-inspector.agent.json

# Build trusted example packages explicitly; reviews never build or pull images.
images:
	mkdir -p bin/container-agent
	CGO_ENABLED=0 GOOS=linux go build -trimpath -o bin/container-agent/diff-inspector ./example/diff-inspector
	docker build -t giad-diff-inspector:example -f example/diff-inspector/Dockerfile bin/container-agent
	docker build -t giad-pr-summary:example example/pr-summary
	docker build -t giad-code-review:example example/code-review
	mkdir -p bin/go-tests
	cp go.mod go.sum example/go-tests/run-tests.sh bin/go-tests/
	docker build -t giad-go-tests:example -f example/go-tests/Dockerfile bin/go-tests

sandbox-smoke:
	GIAD_TEST_DOCKER_IMAGE=giad-pr-summary:example GIAD_TEST_DIFF_IMAGE=giad-diff-inspector:example GIAD_TEST_REVIEW_IMAGE=giad-code-review:example GIAD_TEST_RUNNER_IMAGE=giad-go-tests:example go test -race -count=1 ./internal/sandbox ./internal/agents -run Docker

# Offline integration: launches the actual example through the GIAD broker.
smoke:
	go test -count=1 ./internal/agents -run '^TestDiffInspectorIntegration$$'

check:
	go mod tidy -diff
	go mod verify
	go vet ./...
	go test -race -count=1 ./...

lint:
	npx --yes markdownlint-cli2@0.23.3
