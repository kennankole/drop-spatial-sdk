# drop-spatial-sdk

Vendor-neutral Go SDKs for the spatial infrastructure behind Drop's
routing and delivery tracking (RFC-004). A monorepo of independent
modules, one per capability — `routing/` today, with room for future
siblings (geocoding, tiles) if and when they're actually needed.

Application code depends on each module's interfaces, never on a vendor
package directly, so the underlying engine can be swapped (e.g. OSRM for
Valhalla) without touching business logic.

## Modules

### [`routing/`](routing)

A `Router`, `MapMatcher`, `Matrix`, and `Snapper` port, backed today by an
OSRM implementation (`routing/osrm`), a zero-dependency haversine fallback
for engine outages (`FallbackRouter`), and a curated-path overlay for
areas with poor OSM coverage (`CorridorRouter`).

```go
import (
    "github.com/kennankole/drop-spatial-sdk/routing"
    "github.com/kennankole/drop-spatial-sdk/routing/osrm"
)

primary := osrm.New("http://osrm.internal:5000")
fallback := routing.NewHaversineRouter(1.3, 25)
router := routing.NewFallbackRouter(primary, fallback, myObserver)

route, err := router.Route(ctx, origin, destination)
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for the monorepo's module
conventions.
