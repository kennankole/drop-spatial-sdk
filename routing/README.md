# routing

```
go get github.com/kennankole/drop-spatial-sdk/routing
```

Vendor-neutral ports for road-network computation, plus an OSRM
implementation and a zero-dependency haversine fallback.

## Ports

| Interface | Method | Backs |
|---|---|---|
| `Router` | `Route(ctx, origin, dest, ...opts) (*Route, error)` | Route-once-at-order-creation; customer map overlay |
| `MapMatcher` | `Match(ctx, trace, ...opts) (*MatchedTrace, error)` | Tracking screen's matched trail; the rider-trace speed-model moat |
| `Matrix` | `Table(ctx, sources, dests, ...opts) (*DurationMatrix, error)` | Rider Matching Machine ranking by road-network travel time |
| `Snapper` | `Nearest(ctx, point, n, ...opts) ([]SnappedPoint, error)` | GPS quality gating; pin-drop validation |

`routing.LatLng`, `routing.Route`, `routing.MatchedTrace`, etc. are the
shared vocabulary — application code should import only these plus the
interfaces above everywhere except its composition root.

## OSRM implementation

```go
import "github.com/kennankole/drop-spatial-sdk/routing/osrm"

client := osrm.New("http://osrm.internal:5000",
    osrm.WithTimeout(2*time.Second),
    osrm.WithRetries(1),
    osrm.WithObserver(myObserver),
)
```

`*osrm.Client` implements all four ports. Construct it once, at your
composition root, and pass it around as the interface(s) your code
actually needs (usually just `routing.Router`).

## Error handling

Every port method returns one of the sentinel errors in this package
(`ErrNoRoute`, `ErrNoSegment`, `ErrTooBig`, `ErrInvalidInput`,
`ErrUnavailable`) or a wrapped variant of one — check with `errors.Is`,
never by matching on an error string. `ErrUnavailable` is the only one
that means "the engine itself is unreachable or broken"; the others are
authoritative answers from a healthy engine (no road exists, coordinates
too far from the network, request too large, or bad input).

## Fallback

```go
primary := osrm.New("http://osrm.internal:5000")
fallback := routing.NewHaversineRouter(1.3, 25) // circuity factor, avg km/h
router := routing.NewFallbackRouter(primary, fallback, myObserver)
```

`FallbackRouter` calls `Fallback` only when `Primary` returns
`ErrUnavailable`. It reports every call — including which path was taken —
through `Observer`, so you can alarm on fallback rate without polling
engine health separately.

## Poor-coverage areas: corridors

OSRM only knows the roads in its OSM extract. In areas with thin OSM
coverage — informal settlements, private estate roads, a footpath
riders already use that's never been mapped — OSRM will either return
`ErrNoRoute` or a route that goes nowhere near the real path. `Corridor`
lets ops/product plug specific, known gaps with a curated path, without
touching the OSM graph or redeploying OSRM:

```go
corridor := routing.Corridor{
    ID:   "kibera-toi-path",
    Name: "Toi Market footpath",
    Path: []routing.LatLng{
        {Lat: -1.3130, Lng: 36.7820}, // gateway: a point OSRM can route to
        {Lat: -1.3138, Lng: 36.7825},
        {Lat: -1.3146, Lng: 36.7831},
        {Lat: -1.3154, Lng: 36.7837}, // a known customer landmark, deep inside
    },
    Radius:   100, // meters; how close a query point must be to snap here
    SpeedKMH: 12,  // assumed travel speed along the path
}

router := routing.NewCorridorRouter(osrmClient, []routing.Corridor{corridor}, myObserver)
route, err := router.Route(ctx, riderLocation, customerPinDrop)
```

When the destination (or origin) snaps within `Radius` of the corridor,
`CorridorRouter` asks the underlying router for the leg up to the
corridor's `Gateway` (`Path[0]`) and stitches the curated path onto it;
when both endpoints snap to the same corridor, the underlying router
isn't called at all. Everything else passes through unchanged.

This is deliberately narrow: a hand-curated list of known paths, not a
custom OSM graph. It's meant to close specific, reported gaps — a rider
flags "there's no route to this address," ops walks or rides the actual
path once, and it becomes a `Corridor`. `Corridor.Path` should be dense
enough that consecutive points are no further apart than `Radius`, since
a query snaps to the nearest *vertex*, not the nearest point on a
segment.

If OSM coverage in a zone improves enough that OSRM routes it correctly
on its own, delete the corridor — there's no migration step, since
`CorridorRouter` simply stops matching and falls through.

`CorridorRouter` and `FallbackRouter` compose in either order (both just
implement `Router`), so a deployment can have both engine-outage
resilience and poor-coverage resilience at once.

## Observability

```go
obs := routing.ObserverFunc(func(e routing.Event) {
    metrics.Observe(string(e.Op), e.Latency, e.Err != nil, e.Fallback, e.Overlay)
})
```

Pass an `Observer` via `osrm.WithObserver`, `routing.NewFallbackRouter`, or
`routing.NewCorridorRouter`. Every call — success or failure — produces
exactly one `Event` per engine invoked, so a fallback produces two: one
for the failed primary call, one for the fallback call. `Event.Overlay`
is true when a `CorridorRouter` answered (fully or partly) from curated
corridor data.

## What this module does not do

- No caching. Route-once-and-cache is an application-level decision
  (typically: once at order creation).
- No ETA calibration. `Route.Duration` is the engine's free-flow
  estimate; time-of-day and traffic adjustment stay in application code,
  anchored in your own trace data.
- No Valhalla implementation yet — the ports are engine-agnostic by
  design, so `routing/valhalla` can land later behind the same
  interfaces without call-site changes, if route-shape quality ever
  becomes a measured problem `routing/osrm` can't solve.
