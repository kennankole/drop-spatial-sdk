package osrm

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/kennankole/drop-spatial-sdk/routing"
)

// osrmMatchResponse mirrors the relevant subset of OSRM's /match response:
// https://project-osrm.org/docs/v5.24.0/api/#match-service
type osrmMatchResponse struct {
	Code      string         `json:"code"`
	Matchings []osrmMatching `json:"matchings"`
}

type osrmMatching struct {
	Confidence float64   `json:"confidence"`
	Distance   float64   `json:"distance"`
	Duration   float64   `json:"duration"`
	Geometry   string    `json:"geometry"`
	Legs       []osrmLeg `json:"legs"`
}

// Match implements routing.MapMatcher.
func (c *Client) Match(ctx context.Context, trace []routing.TracePoint, opts ...routing.MatchOption) (*routing.MatchedTrace, error) {
	if len(trace) < 2 {
		return nil, fmt.Errorf("%w: match requires at least 2 trace points, got %d", routing.ErrInvalidInput, len(trace))
	}
	for _, p := range trace {
		if !p.Valid() {
			return nil, routing.ErrInvalidInput
		}
	}

	o := routing.ResolveMatchOptions(opts...)
	if len(o.Radiuses) > 0 && len(o.Radiuses) != len(trace) {
		return nil, fmt.Errorf("%w: radiuses length %d does not match trace length %d", routing.ErrInvalidInput, len(o.Radiuses), len(trace))
	}

	coords := make([]string, len(trace))
	timestamps := make([]string, len(trace))
	radiuses := make([]string, len(trace))
	for i, p := range trace {
		coords[i] = formatCoord(p.LatLng)
		timestamps[i] = strconv.FormatInt(p.Timestamp.Unix(), 10)

		r := p.Accuracy
		if len(o.Radiuses) > 0 {
			r = o.Radiuses[i]
		}
		if r <= 0 {
			r = 5 // meters; OSRM's own default GPS precision assumption
		}
		radiuses[i] = strconv.FormatFloat(r, 'f', 1, 64)
	}

	q := url.Values{}
	q.Set("overview", "full")
	q.Set("geometries", "polyline6")
	q.Set("timestamps", strings.Join(timestamps, ";"))
	q.Set("radiuses", strings.Join(radiuses, ";"))

	reqURL := fmt.Sprintf("%s/match/v1/%s/%s?%s",
		strings.TrimRight(c.baseURL, "/"),
		c.profile,
		strings.Join(coords, ";"),
		q.Encode(),
	)

	start := time.Now()
	var resp osrmMatchResponse
	err := c.doRequest(ctx, reqURL, &resp)
	c.observer.Observe(routing.Event{Op: routing.OpMatch, Latency: time.Since(start), Err: err})
	if err != nil {
		return nil, err
	}

	if len(resp.Matchings) == 0 {
		return nil, routing.ErrNoRoute
	}

	return toMatchedTrace(resp.Matchings[0])
}

func toMatchedTrace(m osrmMatching) (*routing.MatchedTrace, error) {
	geometry, err := decodeGeometry(m.Geometry)
	if err != nil {
		return nil, err
	}

	legs := make([]routing.Leg, len(m.Legs))
	for i, l := range m.Legs {
		legs[i] = routing.Leg{
			Distance: l.Distance,
			Duration: secondsToDuration(l.Duration),
		}
	}

	return &routing.MatchedTrace{
		Confidence: m.Confidence,
		Geometry:   geometry,
		Legs:       legs,
	}, nil
}
