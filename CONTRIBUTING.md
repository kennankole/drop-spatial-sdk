# Contributing

This is a monorepo of independent Go modules, one per spatial capability
(routing today; geocoding, tiles, or others later, if and when RFC-004's
trigger-based sequencing calls for them). Each module has its own
`go.mod`, is released with its own tag prefix, and pulls in only the
dependencies it actually needs.

## Adding a new library

1. Create a top-level directory named after the capability, not the
   vendor (`geocoding/`, not `photon/`). The directory name is the
   package name callers see; naming it after a vendor rebuilds the
   lock-in the adapter pattern exists to avoid.
2. `cd <dir> && go mod init github.com/kennankole/drop-spatial-sdk/<dir>`.
3. Put vendor-neutral ports and types at the module root. Put each
   vendor's implementation in its own subpackage (`<dir>/osrm`,
   `<dir>/photon`). Application code imports the root package for types
   and interfaces, and imports a vendor subpackage only at its
   composition root.
4. Add the module to CI: nothing to do — `.github/workflows/ci.yml`
   discovers every `go.mod` in the repo and runs build/vet/test/lint
   against each.
5. Tag releases as `<dir>/vX.Y.Z` (e.g. `routing/v0.1.0`).

## Conventions

- Stdlib only in vendor-neutral packages, where practical. A shared SDK
  pulled into every service shouldn't impose a dependency tree on
  consumers who only need the types.
- Every exported error a port can return should be a sentinel in that
  module's root package (see `routing.ErrNoRoute`, `routing.ErrUnavailable`,
  etc.), so callers can `errors.Is` against a stable contract instead of
  matching vendor-specific strings.
- Distinguish "the engine gave an authoritative negative answer" (e.g.
  no route exists) from "the engine could not be reached" (`ErrUnavailable`).
  Only the latter is fallback-worthy — see `routing.FallbackRouter`.
- Every network-calling method should notify an `Observer` of its outcome,
  so applications can wire liveness alarms without the SDK depending on
  a specific metrics library.

## Before submitting

From each module directory:

```
gofmt -l .          # must print nothing
go vet ./...
go test -race ./...
golangci-lint run --config ../.golangci.yaml ./...
```

Lint rules live once, at the repo root (`.golangci.yaml`), and apply to
every module — `golangci-lint`'s config already runs `staticcheck` among
its enabled linters, so there's no separate staticcheck step.
