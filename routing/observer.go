package routing

import "time"

// Op identifies which port method an Event describes.
type Op string

const (
	OpRoute   Op = "route"
	OpMatch   Op = "match"
	OpTable   Op = "table"
	OpNearest Op = "nearest"
)

// Event describes the outcome of a single port call, for callers that want
// to wire liveness metrics or alarms (e.g. the zero-rate and fallback
// alarms called for in RFC-004) without the SDK depending on any specific
// metrics library.
type Event struct {
	Op       Op
	Latency  time.Duration
	Err      error
	Fallback bool

	// Overlay is true when a CorridorRouter answered the call (fully or
	// partly) from curated corridor data rather than the underlying
	// engine alone.
	Overlay bool
}

// Observer receives Events from instrumented Router, MapMatcher, Matrix,
// and Snapper implementations.
type Observer interface {
	Observe(Event)
}

// NoopObserver discards all events. It is the default when no Observer is
// configured.
var NoopObserver Observer = noopObserver{}

type noopObserver struct{}

func (noopObserver) Observe(Event) {}

// ObserverFunc adapts a function to the Observer interface.
type ObserverFunc func(Event)

func (f ObserverFunc) Observe(e Event) { f(e) }
