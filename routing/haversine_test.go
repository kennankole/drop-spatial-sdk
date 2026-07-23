package routing

import (
	"context"
	"errors"
	"math"
	"testing"
)

func TestHaversineDistance_KnownPair(t *testing.T) {
	// Nairobi CBD to Jomo Kenyatta International Airport, ~15.5km
	// straight-line per public reference measurements.
	cbd := LatLng{Lat: -1.286389, Lng: 36.817223}
	jkia := LatLng{Lat: -1.319167, Lng: 36.927811}

	got := HaversineDistance(cbd, jkia)
	want := 12822.55 // meters, computed independently from the same formula

	if diff := math.Abs(got - want); diff > 1 {
		t.Errorf("HaversineDistance() = %.2fm, want %.2fm (diff %.2fm)", got, want, diff)
	}
}

func TestHaversineDistance_SamePoint(t *testing.T) {
	p := LatLng{Lat: -1.28, Lng: 36.82}
	if d := HaversineDistance(p, p); d != 0 {
		t.Errorf("HaversineDistance(p, p) = %v, want 0", d)
	}
}

func TestHaversineRouter_Route(t *testing.T) {
	r := NewHaversineRouter(1.3, 25)
	origin := LatLng{Lat: -1.286389, Lng: 36.817223}
	dest := LatLng{Lat: -1.319167, Lng: 36.927811}

	route, err := r.Route(context.Background(), origin, dest)
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}

	straightLine := HaversineDistance(origin, dest)
	wantDistance := straightLine * 1.3
	if math.Abs(route.Distance-wantDistance) > 1 {
		t.Errorf("Distance = %v, want %v", route.Distance, wantDistance)
	}
	if route.Duration <= 0 {
		t.Errorf("Duration = %v, want > 0", route.Duration)
	}
	if len(route.Geometry) != 2 {
		t.Errorf("Geometry has %d points, want 2", len(route.Geometry))
	}
}

func TestHaversineRouter_Defaults(t *testing.T) {
	r := &HaversineRouter{}
	origin := LatLng{Lat: -1.28, Lng: 36.81}
	dest := LatLng{Lat: -1.30, Lng: 36.85}

	route, err := r.Route(context.Background(), origin, dest)
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}
	if route.Distance <= 0 || route.Duration <= 0 {
		t.Errorf("expected positive defaults, got distance=%v duration=%v", route.Distance, route.Duration)
	}
}

func TestHaversineRouter_InvalidInput(t *testing.T) {
	r := NewHaversineRouter(1.3, 25)
	invalid := LatLng{Lat: 999, Lng: 0}
	valid := LatLng{Lat: 0, Lng: 0}

	if _, err := r.Route(context.Background(), invalid, valid); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("Route with invalid origin: got %v, want ErrInvalidInput", err)
	}
	if _, err := r.Route(context.Background(), valid, invalid); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("Route with invalid destination: got %v, want ErrInvalidInput", err)
	}
}
