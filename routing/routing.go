// Package routing defines vendor-neutral ports for road-network
// computation — point-to-point routing, GPS map matching, duration
// matrices, and nearest-road snapping — plus a zero-dependency haversine
// fallback and a decorator that composes a primary engine with a fallback.
//
// Application code should depend only on the interfaces and types in this
// package, never on a specific engine's package (such as routing/osrm)
// outside of the composition root that constructs it. This is what allows
// the underlying engine to be replaced without touching business logic.
package routing

import "context"

// Router computes a route between two points.
type Router interface {
	Route(ctx context.Context, origin, destination LatLng, opts ...RouteOption) (*Route, error)
}

// MapMatcher snaps a GPS trace onto the road network.
type MapMatcher interface {
	Match(ctx context.Context, trace []TracePoint, opts ...MatchOption) (*MatchedTrace, error)
}

// Matrix computes travel durations (and optionally distances) between two
// sets of points.
type Matrix interface {
	Table(ctx context.Context, sources, destinations []LatLng, opts ...TableOption) (*DurationMatrix, error)
}

// Snapper finds the nearest road-network location(s) to a coordinate.
type Snapper interface {
	Nearest(ctx context.Context, point LatLng, n int, opts ...NearestOption) ([]SnappedPoint, error)
}

// RouteOptions holds the resolved options for a Route call. Implementations
// read from this struct rather than iterating RouteOption directly.
type RouteOptions struct {
	// Steps requests turn-by-turn maneuver detail on each Leg.
	Steps bool

	// Alternatives requests up to N alternative routes. Implementations
	// that support alternatives may return them via a mechanism outside
	// the single Route return value in a future version; for v0.1 this
	// is reserved and implementations may ignore it.
	Alternatives int
}

// RouteOption configures a Route call.
type RouteOption func(*RouteOptions)

// WithSteps requests turn-by-turn maneuver detail on the returned route.
func WithSteps(steps bool) RouteOption {
	return func(o *RouteOptions) { o.Steps = steps }
}

// WithAlternatives requests up to n alternative routes, when supported.
func WithAlternatives(n int) RouteOption {
	return func(o *RouteOptions) { o.Alternatives = n }
}

// ResolveRouteOptions applies opts to a zero-valued RouteOptions and
// returns the result. Implementations call this at the top of Route.
func ResolveRouteOptions(opts ...RouteOption) RouteOptions {
	var o RouteOptions
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// MatchOptions holds the resolved options for a Match call.
type MatchOptions struct {
	// Radiuses overrides the per-point GPS accuracy radius (meters)
	// used for snapping. Must be empty or the same length as the trace.
	Radiuses []float64
}

// MatchOption configures a Match call.
type MatchOption func(*MatchOptions)

// WithRadiuses overrides the per-point GPS accuracy radius (meters) used
// for snapping during map matching.
func WithRadiuses(radiuses []float64) MatchOption {
	return func(o *MatchOptions) { o.Radiuses = radiuses }
}

// ResolveMatchOptions applies opts to a zero-valued MatchOptions and
// returns the result.
func ResolveMatchOptions(opts ...MatchOption) MatchOptions {
	var o MatchOptions
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// TableOptions holds the resolved options for a Table call.
type TableOptions struct {
	// WithDistances requests the Distances field be populated in
	// addition to Durations.
	WithDistances bool
}

// TableOption configures a Table call.
type TableOption func(*TableOptions)

// WithDistances requests that a Table call also populate DurationMatrix.Distances.
func WithDistances(want bool) TableOption {
	return func(o *TableOptions) { o.WithDistances = want }
}

// ResolveTableOptions applies opts to a zero-valued TableOptions and
// returns the result.
func ResolveTableOptions(opts ...TableOption) TableOptions {
	var o TableOptions
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// NearestOptions holds the resolved options for a Nearest call.
type NearestOptions struct{}

// NearestOption configures a Nearest call. Reserved for future use (e.g.
// bearing filters); no options are defined in v0.1.
type NearestOption func(*NearestOptions)

// ResolveNearestOptions applies opts to a zero-valued NearestOptions and
// returns the result.
func ResolveNearestOptions(opts ...NearestOption) NearestOptions {
	var o NearestOptions
	for _, opt := range opts {
		opt(&o)
	}
	return o
}
