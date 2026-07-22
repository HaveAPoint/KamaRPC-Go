# Repository Guidelines

## Project Structure & Module Organization

This is a Go RPC implementation (`module kamaRPC`, Go 1.25.4). Keep reusable service types in `pkg/api/`, the only area intended for external use. Core code belongs in `internal/`, organized by responsibility: `client`, `server`, `transport`, `protocol`, `codec`, `registry`, `loadbalance`, `limiter`, and `breaker`. Programs under `cmd/` include RPC examples (`server1`, `server2`, `client`) and load tools (`bench1`, `bench2`). Add tests beside their package as `*_test.go`; keep fixtures in `testdata/`.

## Build, Test, and Development Commands

- `go mod download` installs the versions pinned in `go.mod` and `go.sum`.
- `go build ./...` compiles every package and command without leaving local binaries.
- `go test ./...` runs the full test suite. The repository currently has no checked-in tests, so new behavior should add them.
- `go test -race ./...` checks concurrent client, pool, limiter, and transport code for data races.
- `go vet ./...` performs standard static checks.
- `gofmt -w .` formats all Go files; review the resulting diff before committing.

Local examples require etcd at `localhost:2379`. In separate terminals, run `go run ./cmd/server1` and then `go run ./cmd/client`. Use `go run ./cmd/bench2 -c 50 -d 10` for a duration-based benchmark.

## Coding Style & Naming Conventions

Follow idiomatic Go and let `gofmt` control indentation and imports. Use lowercase, single-word package names; MixedCaps identifiers; short, consistent receiver names; `ErrName` for sentinel errors; and established constructors for exported types. Keep `cmd/` focused on wiring and flags, with protocol or business logic in `internal/` or `pkg/`. Preserve functional-option patterns such as `WithClientTimeout` and `WithServerCodec`. Wrap errors using `%w` when callers may inspect them.

## Testing Guidelines

Use the standard `testing` package and table-driven tests with named subtests. Name tests `TestFunction` or `TestType_Method`, benchmarks `BenchmarkName`, and fuzz targets `FuzzName`. Prefer deterministic unit tests; tests requiring etcd or TCP listeners should isolate resources and document prerequisites. Run both `go test ./...` and `go test -race ./...` before opening a PR. No coverage threshold is currently enforced.

## Commit & Pull Request Guidelines

Recent commits use short Chinese summaries such as `更新性能测试文件` and `删除冗余文件`. Keep commits concise, imperative, and limited to one logical change. PRs should explain the behavior changed, list validation commands, link related issues, and include benchmark before/after results for performance-sensitive changes. Do not commit generated executables under `cmd/`.
