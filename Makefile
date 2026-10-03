.PHONY: all build example smoke check lint

all: build example

build:
	go build -trimpath -o bin/giad ./cmd/giad

example:
	go build -trimpath -o bin/diff-inspector ./example/diff-inspector
	./bin/diff-inspector --manifest > bin/diff-inspector.agent.json

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
