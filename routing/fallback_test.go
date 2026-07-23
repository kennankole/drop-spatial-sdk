package routing

import (
	"context"
	"errors"
	"testing"
)

type stubRouter struct {
	route *Route
	err   error
	calls int
}

func (s *stubRouter) Route(_ context.Context, _, _ LatLng, _ ...RouteOption) (*Route, error) {
	s.calls++
	return s.route, s.err
}

func TestFallbackRouter_PrimarySucceeds(t *testing.T) {
	primary := &stubRouter{route: &Route{Distance: 100}}
	fallback := &stubRouter{route: &Route{Distance: 999}}

	fr := NewFallbackRouter(primary, fallback, nil)
	route, err := fr.Route(context.Background(), LatLng{}, LatLng{})
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}
	if route.Distance != 100 {
		t.Errorf("Distance = %v, want 100 (primary's result)", route.Distance)
	}
	if fallback.calls != 0 {
		t.Errorf("fallback was called %d times, want 0", fallback.calls)
	}
}

func TestFallbackRouter_FallsBackOnUnavailable(t *testing.T) {
	primary := &stubRouter{err: ErrUnavailable}
	fallback := &stubRouter{route: &Route{Distance: 999}}

	var events []Event
	obs := ObserverFunc(func(e Event) { events = append(events, e) })

	fr := NewFallbackRouter(primary, fallback, obs)
	route, err := fr.Route(context.Background(), LatLng{}, LatLng{})
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}
	if route.Distance != 999 {
		t.Errorf("Distance = %v, want 999 (fallback's result)", route.Distance)
	}
	if fallback.calls != 1 {
		t.Errorf("fallback was called %d times, want 1", fallback.calls)
	}

	if len(events) != 2 {
		t.Fatalf("got %d observer events, want 2", len(events))
	}
	if !events[1].Fallback {
		t.Errorf("second event Fallback = false, want true")
	}
}

func TestFallbackRouter_NoFallbackOnSemanticError(t *testing.T) {
	primary := &stubRouter{err: ErrNoRoute}
	fallback := &stubRouter{route: &Route{Distance: 999}}

	fr := NewFallbackRouter(primary, fallback, nil)
	_, err := fr.Route(context.Background(), LatLng{}, LatLng{})
	if !errors.Is(err, ErrNoRoute) {
		t.Errorf("Route error = %v, want ErrNoRoute", err)
	}
	if fallback.calls != 0 {
		t.Errorf("fallback was called %d times, want 0 (ErrNoRoute is not fallback-worthy)", fallback.calls)
	}
}
