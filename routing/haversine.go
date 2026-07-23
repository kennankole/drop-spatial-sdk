package routing

import (
	"context"
	"math"
	"time"
)

const earthRadiusMeters = 6371000.0

var _ Router = (*HaversineRouter)(nil)

// HaversineRouter is a zero-dependency Router that estimates distance via
// the great-circle formula and duration from a configurable average speed.
// It has no notion of the road network, so it is not a substitute for a
// real routing engine — it exists as the resilience backstop a FallbackRouter
// falls back to when the primary engine is unavailable, matching the
// existing haversine fallback already in use in the backend today.
type HaversineRouter struct {
	// CircuityFactor scales the great-circle distance up to approximate
	// real road distance, which is never a straight line. 1.0 means no
	// adjustment. Typical urban road networks run 1.2-1.4x straight-line
	// distance; defaults to 1.3 if zero.
	CircuityFactor float64

	// AverageSpeedKMH is the assumed travel speed used to derive a
	// duration estimate from the (circuity-adjusted) distance. Defaults
	// to 25 km/h if zero, a conservative urban estimate.
	AverageSpeedKMH float64
}

// NewHaversineRouter returns a HaversineRouter with the given circuity
// factor and average speed. Passing 0 for either uses the documented
// default.
func NewHaversineRouter(circuityFactor, averageSpeedKMH float64) *HaversineRouter {
	return &HaversineRouter{CircuityFactor: circuityFactor, AverageSpeedKMH: averageSpeedKMH}
}

// Route implements Router. It ignores opts: there are no turn-by-turn
// steps or alternatives to compute over a straight line.
func (h *HaversineRouter) Route(_ context.Context, origin, destination LatLng, _ ...RouteOption) (*Route, error) {
	if !origin.Valid() || !destination.Valid() {
		return nil, ErrInvalidInput
	}

	circuity := h.CircuityFactor
	if circuity == 0 {
		circuity = 1.3
	}
	speed := h.AverageSpeedKMH
	if speed == 0 {
		speed = 25
	}

	straightLine := HaversineDistance(origin, destination)
	distance := straightLine * circuity
	durationHours := (distance / 1000) / speed
	duration := time.Duration(durationHours * float64(time.Hour))

	return &Route{
		Distance: distance,
		Duration: duration,
		Geometry: []LatLng{origin, destination},
		Legs: []Leg{
			{Distance: distance, Duration: duration},
		},
	}, nil
}

// HaversineDistance returns the great-circle distance between a and b in
// meters.
func HaversineDistance(a, b LatLng) float64 {
	lat1 := a.Lat * math.Pi / 180
	lat2 := b.Lat * math.Pi / 180
	dLat := (b.Lat - a.Lat) * math.Pi / 180
	dLng := (b.Lng - a.Lng) * math.Pi / 180

	sinDLat := math.Sin(dLat / 2)
	sinDLng := math.Sin(dLng / 2)

	x := sinDLat*sinDLat + math.Cos(lat1)*math.Cos(lat2)*sinDLng*sinDLng
	c := 2 * math.Atan2(math.Sqrt(x), math.Sqrt(1-x))

	return earthRadiusMeters * c
}
