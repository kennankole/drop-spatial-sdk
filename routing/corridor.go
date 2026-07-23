package routing

import (
	"context"
	"math"
	"time"
)

// Corridor is a manually curated path connecting a point on the
// underlying engine's routable network (its Gateway, i.e. Path[0]) to one
// or more destinations inside an area with poor OSM coverage — an
// informal settlement footpath, a private estate access road, a newly
// opened road not yet mapped, and so on.
//
// Corridors are curated by ops/product from ground truth (rider-reported
// paths, a one-off survey walk, a known landmark route), not derived from
// OSM. They exist to plug specific, known gaps — not to replace the road
// network wholesale.
//
// Path points should be dense enough that no gap between consecutive
// points exceeds Radius: a query point snaps to the nearest Path vertex,
// not to the nearest point on a Path segment, so sparse waypoints make
// the snap (and the resulting distance/duration estimate) coarser.
type Corridor struct {
	ID   string
	Name string

	// Path is the ordered sequence of waypoints from the gateway
	// (Path[0]) to the far end of the corridor. Must have at least 2
	// points; corridors with fewer are ignored by CorridorRouter.
	Path []LatLng

	// Radius is the snap distance in meters: a query point is considered
	// "on this corridor" when it falls within Radius of the nearest
	// point in Path. Corridors with Radius <= 0 are ignored by
	// CorridorRouter.
	Radius float64

	// SpeedKMH is the assumed travel speed along this corridor (e.g.
	// ~10-15 km/h for a footpath a boda boda can use). Defaults to 10 if
	// zero.
	SpeedKMH float64
}

// Gateway returns the corridor's entry point on the underlying router's
// network.
func (c Corridor) Gateway() LatLng { return c.Path[0] }

func (c Corridor) valid() bool {
	return len(c.Path) >= 2 && c.Radius > 0
}

// nearestIndex returns the index of the Path point closest to p and the
// distance to it in meters.
func (c Corridor) nearestIndex(p LatLng) (index int, distance float64) {
	bestIdx := 0
	bestDist := HaversineDistance(p, c.Path[0])
	for i := 1; i < len(c.Path); i++ {
		d := HaversineDistance(p, c.Path[i])
		if d < bestDist {
			bestDist = d
			bestIdx = i
		}
	}
	return bestIdx, bestDist
}

// segment returns the distance, duration, and geometry of the corridor
// sub-path from index from to index to, inclusive, walked in whichever
// direction (from may be greater than to).
func (c Corridor) segment(from, to int) (distance float64, duration time.Duration, geometry []LatLng) {
	step := 1
	if to < from {
		step = -1
	}

	geometry = []LatLng{c.Path[from]}
	for i := from; i != to; i += step {
		distance += HaversineDistance(c.Path[i], c.Path[i+step])
		geometry = append(geometry, c.Path[i+step])
	}

	speed := c.SpeedKMH
	if speed == 0 {
		speed = 10
	}
	hours := (distance / 1000) / speed
	duration = time.Duration(hours * float64(time.Hour))
	return distance, duration, geometry
}

// CorridorRouter composes an underlying Router with a set of curated
// Corridors covering pockets the underlying engine's graph doesn't reach.
//
//   - When both origin and destination snap to the same Corridor, the
//     underlying Router is not called at all: the corridor's own sub-path
//     between the two points is authoritative.
//   - When only one endpoint snaps to a Corridor, Underlying computes the
//     leg between the other endpoint and the corridor's Gateway, and that
//     leg is stitched together with the corridor sub-path from the
//     Gateway to the snapped point.
//   - When neither endpoint snaps to any Corridor (or they snap to two
//     different Corridors — routing between two distinct unmapped
//     pockets is out of scope), the call passes through to Underlying
//     unchanged.
//
// CorridorRouter can wrap a FallbackRouter (or vice versa) to combine
// engine-outage resilience with poor-coverage resilience.
type CorridorRouter struct {
	Underlying Router
	Corridors  []Corridor
	Observer   Observer
}

// NewCorridorRouter returns a CorridorRouter. obs may be nil, in which
// case events are discarded.
func NewCorridorRouter(underlying Router, corridors []Corridor, obs Observer) *CorridorRouter {
	if obs == nil {
		obs = NoopObserver
	}
	return &CorridorRouter{Underlying: underlying, Corridors: corridors, Observer: obs}
}

var _ Router = (*CorridorRouter)(nil)

// Route implements Router.
func (c *CorridorRouter) Route(ctx context.Context, origin, destination LatLng, opts ...RouteOption) (*Route, error) {
	start := time.Now()

	originCorridor, originIdx, originMatched := c.match(origin)
	destCorridor, destIdx, destMatched := c.match(destination)

	var route *Route
	var err error
	overlay := false

	switch {
	case originMatched && destMatched && originCorridor.ID == destCorridor.ID:
		distance, duration, geometry := originCorridor.segment(originIdx, destIdx)
		route = &Route{
			Distance: distance,
			Duration: duration,
			Geometry: geometry,
			Legs:     []Leg{{Distance: distance, Duration: duration}},
		}
		overlay = true

	case destMatched && !originMatched:
		var leg *Route
		leg, err = c.Underlying.Route(ctx, origin, destCorridor.Gateway(), opts...)
		if err == nil {
			distance, duration, geometry := destCorridor.segment(0, destIdx)
			route = appendSegment(leg, distance, duration, geometry)
			overlay = true
		}

	case originMatched && !destMatched:
		var leg *Route
		leg, err = c.Underlying.Route(ctx, originCorridor.Gateway(), destination, opts...)
		if err == nil {
			distance, duration, geometry := originCorridor.segment(originIdx, 0)
			route = prependSegment(distance, duration, geometry, leg)
			overlay = true
		}

	default:
		route, err = c.Underlying.Route(ctx, origin, destination, opts...)
	}

	c.Observer.Observe(Event{Op: OpRoute, Latency: time.Since(start), Err: err, Overlay: overlay})
	return route, err
}

// match returns the Corridor whose nearest Path point to p is closest
// among all Corridors within their own Radius, or ok=false if none
// qualify.
func (c *CorridorRouter) match(p LatLng) (corridor Corridor, index int, ok bool) {
	bestDist := math.MaxFloat64
	bestIdx := -1
	var best Corridor

	for _, corr := range c.Corridors {
		if !corr.valid() {
			continue
		}
		idx, dist := corr.nearestIndex(p)
		if dist <= corr.Radius && dist < bestDist {
			best, bestIdx, bestDist, ok = corr, idx, dist, true
		}
	}
	return best, bestIdx, ok
}

// appendSegment concatenates a corridor sub-path onto the end of base,
// which arrives at the corridor's Gateway (base's last geometry point).
// The duplicate junction point is dropped from the appended geometry.
func appendSegment(base *Route, distance float64, duration time.Duration, geometry []LatLng) *Route {
	fullGeometry := base.Geometry
	if len(geometry) > 1 {
		fullGeometry = append(append([]LatLng{}, base.Geometry...), geometry[1:]...)
	}
	return &Route{
		Distance: base.Distance + distance,
		Duration: base.Duration + duration,
		Geometry: fullGeometry,
		Legs:     append(append([]Leg{}, base.Legs...), Leg{Distance: distance, Duration: duration}),
	}
}

// prependSegment concatenates a corridor sub-path onto the front of base,
// which departs from the corridor's Gateway (base's first geometry
// point). The duplicate junction point is dropped from the prepended
// geometry.
func prependSegment(distance float64, duration time.Duration, geometry []LatLng, base *Route) *Route {
	fullGeometry := geometry
	if len(base.Geometry) > 1 {
		fullGeometry = append(append([]LatLng{}, geometry...), base.Geometry[1:]...)
	}
	return &Route{
		Distance: distance + base.Distance,
		Duration: duration + base.Duration,
		Geometry: fullGeometry,
		Legs:     append([]Leg{{Distance: distance, Duration: duration}}, base.Legs...),
	}
}
