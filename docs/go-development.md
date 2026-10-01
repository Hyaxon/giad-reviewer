# Go orientation and development

## What the files mean

| File | Meaning |
| --- | --- |
| `go.mod` | Module import path, Go version, and dependency requirements |
| `go.sum` | Dependency checksums maintained by Go; not a general lockfile |
| `cmd/magi/main.go` | Executable entry point |
| `internal/*/doc.go` | Package documentation; currently placeholders for future code |
| `magi.example.toml` | Example application data; not executable Go code |
| `bin/magi` | Locally built executable; excluded from Git |

A **module** is the collection of Go packages described by `go.mod`. A **package**
groups related Go files, normally in one directory. An **import** lets one package
use exported names from another. Names beginning with a capital letter are exported.

`package main` builds an executable, and `func main()` is its entry point. A
package named `review`, for example, provides functionality used by other packages
instead of starting a program itself. Go's `internal` directory rule limits
imports to code within the allowed parent tree.

The current `main` function constructs a Cobra command and calls `Execute()`.
Cobra parses arguments and handles help/version output. If execution returns an
error, `main` prints it to standard error and exits unsuccessfully.

`version` is currently a string set to `dev`; it is not the Go compiler version.

## Commands that work today

Run these from the project directory:

```sh
go mod download
go run ./cmd/magi --help
go run ./cmd/magi --version
go build -o bin/magi ./cmd/magi
./bin/magi --help
```

`go run` compiles and launches the program in one step. `go build` creates an
executable that you can run later. A completed build does not leave a service
running. Our current help/version commands exit after printing their output.

For normal development checks:

```sh
gofmt -w cmd internal
go test ./...
go vet ./...
```

`gofmt` edits formatting. `go test ./...` checks all packages and runs any tests;
there currently are none. `go vet` finds certain suspicious code patterns, but
does not prove correctness. Add focused tests when real behavior is introduced.

`go get` adds or changes dependency requirements. `go mod tidy` reconciles those
requirements with imports and removes unused dependencies. Add the PDF's optional
JWT/TOML libraries when implementing their consumers, rather than adding blank
imports just to retain dependencies.

## Processes and stopping work

Control+C interrupts a foreground build or program. A background process must
be identified and stopped separately; record its PID when starting a service.
Do not assume every Go-related process is MAGI.

`gopls` is the editor's Go language server. It supplies navigation, diagnostics,
and completion, and can remain running even when no MAGI command is active.
The editor may restart it if it is killed. Disable the Go extension for the
workspace if you intentionally want that tooling stopped.

## Concepts the implementation will introduce

- `struct`: a typed group of fields, such as a finding or configuration.
- `interface`: a contract allowing the runner to use interchangeable model providers.
- `context.Context`: cancellation and deadlines passed through API/model/tool calls.
- `error`: an explicit failure result callers must handle.
- JSON and TOML tags: mapping Go field names to serialized configuration/API fields.

Start with configuration and read-only GitHub calls. There is no need to learn
worker queues, goroutines, or distributed scheduling before that first slice.
