package routing

import (
	"errors"
	"time"
)

// Sentinel errors returned by Router, MapMatcher, Matrix, and Snapper
// implementations. Callers should use errors.Is against these rather than
// matching on implementation-specific error strings, so that swapping the
// underlying engine (e.g. OSRM for Valhalla) does not change error-handling
// call sites.
var (
	// ErrNoRoute means the engine could not find any path between the
	// requested points (e.g. disconnected graph components).
	ErrNoRoute = errors.New("routing: no route found")

	// ErrNoSegment means one or more input coordinates could not be
	// snapped to the road network within tolerance.
	ErrNoSegment = errors.New("routing: no road segment near input coordinate")

	// ErrTooBig means the request exceeds the engine's size limits
	// (e.g. too many waypoints in a match or table request).
	ErrTooBig = errors.New("routing: request exceeds engine limits")

	// ErrInvalidInput means the caller supplied malformed input
	// (out-of-range coordinates, empty trace, etc.) that was rejected
	// before any network call was made.
	ErrInvalidInput = errors.New("routing: invalid input")

	// ErrUnavailable means the engine could not be reached or returned
	// a transport-level failure (timeout, connection refused, 5xx).
	// This is the error FallbackRouter treats as fallback-worthy.
	ErrUnavailable = errors.New("routing: engine unavailable")
)

// LatLng is a WGS84 coordinate.
type LatLng struct {
	Lat float64
	Lng float64
}

// Valid reports whether l falls within the legal WGS84 coordinate range.
func (l LatLng) Valid() bool {
	return l.Lat >= -90 && l.Lat <= 90 && l.Lng >= -180 && l.Lng <= 180
}

// Route is a computed path between an origin and a destination.
type Route struct {
	// Distance is the total route length in meters.
	Distance float64

	// Duration is the engine's free-flow travel time estimate. It does
	// not account for traffic; callers that need traffic-aware ETAs
	// apply their own calibration on top of this value.
	Duration time.Duration

	// Geometry is the route path, decoded to coordinates in order from
	// origin to destination.
	Geometry []LatLng

	// Legs breaks the route down by waypoint (present when more than
	// two coordinates were requested; a plain origin/destination route
	// has exactly one leg).
	Legs []Leg
}

// Leg is one origin-to-waypoint (or waypoint-to-destination) segment of a
// Route.
type Leg struct {
	Distance float64
	Duration time.Duration
	Steps    []Step
}

// Step is a single maneuver within a Leg (turn-by-turn instruction unit).
// Steps are only populated when requested via WithSteps.
type Step struct {
	Distance     float64
	Duration     time.Duration
	Geometry     []LatLng
	Name         string
	ManeuverType string
	Modifier     string
}

// TracePoint is one observation in a GPS trace submitted for map matching.
type TracePoint struct {
	LatLng
	Timestamp time.Time

	// Accuracy is the GPS accuracy radius in meters, when known. Zero
	// means unknown; implementations fall back to a default.
	Accuracy float64
}

// MatchedTrace is the result of snapping a GPS trace onto the road network.
type MatchedTrace struct {
	// Confidence is the engine's match quality score in [0,1].
	Confidence float64

	// Geometry is the road-snapped path.
	Geometry []LatLng

	Legs []Leg
}

// SnappedPoint is a road-network location near a queried coordinate.
type SnappedPoint struct {
	LatLng

	// Distance is the distance in meters from the queried point to this
	// snapped location.
	Distance float64

	// Name is the road name, when available.
	Name string
}

// DurationMatrix is the result of a one-to-many (or many-to-many) travel
// time query.
type DurationMatrix struct {
	Sources      []LatLng
	Destinations []LatLng

	// Durations[i][j] is the travel time from Sources[i] to
	// Destinations[j]. A negative value means no route was found for
	// that pair.
	Durations [][]time.Duration

	// Distances[i][j] is the travel distance in meters from Sources[i]
	// to Destinations[j], mirroring Durations. Nil if the engine did
	// not return distances.
	Distances [][]float64
}
