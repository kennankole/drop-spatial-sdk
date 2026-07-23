package routing

import (
	"context"
	"errors"
	"time"
)

// FallbackRouter composes a primary Router with a fallback Router. It
// calls Fallback only when Primary returns ErrUnavailable — a transport or
// engine-health failure — not for semantic answers like ErrNoRoute or
// ErrNoSegment, which mean the primary engine is up and has already given
// its authoritative answer.
//
// Every call is reported to Observer (if set), with Event.Fallback true
// when the fallback path was taken, so callers can wire the fallback-rate
// alarms called for in RFC-004 without polling engine health separately.
type FallbackRouter struct {
	Primary  Router
	Fallback Router
	Observer Observer
}

var _ Router = (*FallbackRouter)(nil)

// NewFallbackRouter returns a FallbackRouter. obs may be nil, in which
// case events are discarded.
func NewFallbackRouter(primary, fallback Router, obs Observer) *FallbackRouter {
	if obs == nil {
		obs = NoopObserver
	}
	return &FallbackRouter{Primary: primary, Fallback: fallback, Observer: obs}
}

// Route implements Router.
func (f *FallbackRouter) Route(ctx context.Context, origin, destination LatLng, opts ...RouteOption) (*Route, error) {
	start := time.Now()
	route, err := f.Primary.Route(ctx, origin, destination, opts...)
	if err == nil {
		f.Observer.Observe(Event{Op: OpRoute, Latency: time.Since(start), Fallback: false})
		return route, nil
	}

	f.Observer.Observe(Event{Op: OpRoute, Latency: time.Since(start), Err: err, Fallback: false})

	if !errors.Is(err, ErrUnavailable) {
		return nil, err
	}

	fbStart := time.Now()
	route, fbErr := f.Fallback.Route(ctx, origin, destination, opts...)
	f.Observer.Observe(Event{Op: OpRoute, Latency: time.Since(fbStart), Err: fbErr, Fallback: true})
	if fbErr != nil {
		return nil, fbErr
	}
	return route, nil
}
