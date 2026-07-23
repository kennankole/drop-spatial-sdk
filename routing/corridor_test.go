package routing

import (
	"context"
	"errors"
	"testing"
	"time"
)

// A short curated footpath: gateway on the mapped network, running deeper
// into an (imaginary) poorly-mapped pocket.
func sampleCorridor() Corridor {
	return Corridor{
		ID:   "kibera-toi-path",
		Name: "Toi Market footpath",
		Path: []LatLng{
			{Lat: -1.3130, Lng: 36.7820}, // gateway, on the mapped network
			{Lat: -1.3138, Lng: 36.7825},
			{Lat: -1.3146, Lng: 36.7831},
			{Lat: -1.3154, Lng: 36.7837}, // deep end
		},
		Radius:   100,
		SpeedKMH: 12,
	}
}

func TestCorridorRouter_NoMatch_PassesThrough(t *testing.T) {
	underlying := &stubRouter{route: &Route{Distance: 500, Duration: 60}}
	cr := NewCorridorRouter(underlying, []Corridor{sampleCorridor()}, nil)

	farOrigin := LatLng{Lat: -1.28, Lng: 36.81}
	farDest := LatLng{Lat: -1.29, Lng: 36.82}

	route, err := cr.Route(context.Background(), farOrigin, farDest)
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}
	if route.Distance != 500 {
		t.Errorf("Distance = %v, want 500 (passthrough)", route.Distance)
	}
	if underlying.calls != 1 {
		t.Errorf("underlying was called %d times, want 1", underlying.calls)
	}
}

func TestCorridorRouter_DestinationMatches_Stitches(t *testing.T) {
	corridor := sampleCorridor()
	underlying := &stubRouter{route: &Route{
		Distance: 1000,
		Duration: 120 * time.Second,
		Geometry: []LatLng{{Lat: -1.30, Lng: 36.77}, corridor.Gateway()},
	}}

	var events []Event
	obs := ObserverFunc(func(e Event) { events = append(events, e) })
	cr := NewCorridorRouter(underlying, []Corridor{corridor}, obs)

	origin := LatLng{Lat: -1.30, Lng: 36.77} // far from corridor, on mapped network
	dest := corridor.Path[3]                 // deep inside the corridor

	route, err := cr.Route(context.Background(), origin, dest)
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}
	if underlying.calls != 1 {
		t.Fatalf("underlying was called %d times, want 1", underlying.calls)
	}

	// Underlying's leg (1000m) plus the corridor's own distance from
	// Gateway to Path[3].
	corridorDistance, _, _ := corridor.segment(0, 3)
	wantDistance := 1000 + corridorDistance
	if route.Distance != wantDistance {
		t.Errorf("Distance = %v, want %v", route.Distance, wantDistance)
	}

	// Geometry must not duplicate the junction point (the Gateway).
	wantPoints := len(underlying.route.Geometry) + 3 // corridor path minus its own first point
	if len(route.Geometry) != wantPoints {
		t.Errorf("Geometry has %d points, want %d (no duplicated junction)", len(route.Geometry), wantPoints)
	}
	if route.Geometry[len(underlying.route.Geometry)-1] != corridor.Gateway() {
		t.Errorf("Geometry does not transition through the Gateway at the expected index")
	}

	if len(events) != 1 || !events[0].Overlay {
		t.Errorf("expected exactly one Overlay=true event, got %+v", events)
	}
}

func TestCorridorRouter_OriginMatches_Stitches(t *testing.T) {
	corridor := sampleCorridor()
	underlying := &stubRouter{route: &Route{
		Distance: 800,
		Geometry: []LatLng{corridor.Gateway(), {Lat: -1.30, Lng: 36.77}},
	}}
	cr := NewCorridorRouter(underlying, []Corridor{corridor}, nil)

	origin := corridor.Path[3] // deep inside the corridor
	dest := LatLng{Lat: -1.30, Lng: 36.77}

	route, err := cr.Route(context.Background(), origin, dest)
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}
	corridorDistance, _, _ := corridor.segment(3, 0)
	wantDistance := corridorDistance + 800
	if route.Distance != wantDistance {
		t.Errorf("Distance = %v, want %v", route.Distance, wantDistance)
	}
	if route.Geometry[0] != corridor.Path[3] {
		t.Errorf("Geometry[0] = %v, want corridor.Path[3] %v", route.Geometry[0], corridor.Path[3])
	}
}

func TestCorridorRouter_BothMatchSameCorridor_SkipsUnderlying(t *testing.T) {
	corridor := sampleCorridor()
	underlying := &stubRouter{route: &Route{Distance: 99999}}
	cr := NewCorridorRouter(underlying, []Corridor{corridor}, nil)

	route, err := cr.Route(context.Background(), corridor.Path[0], corridor.Path[3])
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}
	if underlying.calls != 0 {
		t.Errorf("underlying was called %d times, want 0 (both endpoints resolved within one corridor)", underlying.calls)
	}
	wantDistance, wantDuration, wantGeometry := corridor.segment(0, 3)
	if route.Distance != wantDistance || route.Duration != wantDuration {
		t.Errorf("Distance/Duration = %v/%v, want %v/%v", route.Distance, route.Duration, wantDistance, wantDuration)
	}
	if len(route.Geometry) != len(wantGeometry) {
		t.Errorf("Geometry has %d points, want %d", len(route.Geometry), len(wantGeometry))
	}
}

func TestCorridorRouter_DifferentCorridors_PassesThrough(t *testing.T) {
	corridorA := sampleCorridor()
	corridorB := Corridor{
		ID:     "other-corridor",
		Path:   []LatLng{{Lat: -1.20, Lng: 36.90}, {Lat: -1.205, Lng: 36.905}},
		Radius: 100,
	}
	underlying := &stubRouter{route: &Route{Distance: 4242}}
	cr := NewCorridorRouter(underlying, []Corridor{corridorA, corridorB}, nil)

	route, err := cr.Route(context.Background(), corridorA.Path[0], corridorB.Path[0])
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}
	if route.Distance != 4242 {
		t.Errorf("Distance = %v, want 4242 (passthrough for cross-corridor query)", route.Distance)
	}
	if underlying.calls != 1 {
		t.Errorf("underlying was called %d times, want 1", underlying.calls)
	}
}

func TestCorridorRouter_UnderlyingFailure_Propagates(t *testing.T) {
	corridor := sampleCorridor()
	underlying := &stubRouter{err: ErrUnavailable}
	cr := NewCorridorRouter(underlying, []Corridor{corridor}, nil)

	origin := LatLng{Lat: -1.30, Lng: 36.77}
	dest := corridor.Path[3]

	_, err := cr.Route(context.Background(), origin, dest)
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("Route error = %v, want ErrUnavailable", err)
	}
}

func TestCorridorRouter_InvalidCorridorIgnored(t *testing.T) {
	tooShort := Corridor{ID: "bad", Path: []LatLng{{Lat: -1.31, Lng: 36.78}}, Radius: 100}
	zeroRadius := Corridor{ID: "bad2", Path: []LatLng{{Lat: -1.31, Lng: 36.78}, {Lat: -1.32, Lng: 36.79}}, Radius: 0}

	underlying := &stubRouter{route: &Route{Distance: 1}}
	cr := NewCorridorRouter(underlying, []Corridor{tooShort, zeroRadius}, nil)

	_, err := cr.Route(context.Background(), LatLng{Lat: -1.31, Lng: 36.78}, LatLng{Lat: -1.32, Lng: 36.79})
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}
	if underlying.calls != 1 {
		t.Errorf("underlying was called %d times, want 1 (both corridors should be ignored as invalid)", underlying.calls)
	}
}
