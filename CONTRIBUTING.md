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
5. Releases are automatic (see below) — the first merge to `main` that
   touches the new module's directory cuts its initial `v0.1.0` tag.

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

## Releases

Every push to `main` runs `.github/scripts/release.sh`, which tags and
releases each module independently based on the commits since its last
`<dir>/vX.Y.Z` tag that touched files under that module's directory.
There's no manual tagging step and no release PR to merge — the commit
messages on `main` are what decide what ships and how the version bumps,
so they need a [Conventional Commits](https://www.conventionalcommits.org/)
subject line:

| Subject prefix | Effect |
|---|---|
| `fix:`, `perf:` | patch bump |
| `feat:` | minor bump |
| `!` after the type/scope (e.g. `feat!:`), or a `BREAKING CHANGE:` footer | major bump |
| anything else (`docs:`, `chore:`, `refactor:`, `test:`, `ci:`, ...) | no release on its own |

A module with commits of several kinds since its last tag gets the
highest-priority bump among them (major beats minor beats patch) — the
lower-priority commits ride along in that release rather than being
skipped. A module with no prior tag at all gets an initial `v0.1.0`
unconditionally on its first qualifying merge, regardless of commit type.

Because this runs with no review step before the tag goes out, get the
commit type right — a `fix:` that should have been a `feat!:` still ships,
just under the wrong version number. Preview what a push to `main` would
do without creating anything:

```
.github/scripts/release.sh --dry-run
```

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
