package osrm

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kennankole/drop-spatial-sdk/routing"
)

// osrmNearestResponse mirrors the relevant subset of OSRM's /nearest
// response: https://project-osrm.org/docs/v5.24.0/api/#nearest-service
type osrmNearestResponse struct {
	Code      string             `json:"code"`
	Waypoints []osrmNearestPoint `json:"waypoints"`
}

type osrmNearestPoint struct {
	Location [2]float64 `json:"location"` // [lng, lat]
	Distance float64    `json:"distance"`
	Name     string     `json:"name"`
}

// Nearest implements routing.Snapper.
func (c *Client) Nearest(ctx context.Context, point routing.LatLng, n int, opts ...routing.NearestOption) ([]routing.SnappedPoint, error) {
	if !point.Valid() {
		return nil, routing.ErrInvalidInput
	}
	if n < 1 {
		return nil, fmt.Errorf("%w: n must be >= 1, got %d", routing.ErrInvalidInput, n)
	}
	routing.ResolveNearestOptions(opts...)

	reqURL := fmt.Sprintf("%s/nearest/v1/%s/%s?number=%s",
		strings.TrimRight(c.baseURL, "/"),
		c.profile,
		formatCoord(point),
		strconv.Itoa(n),
	)

	start := time.Now()
	var resp osrmNearestResponse
	err := c.doRequest(ctx, reqURL, &resp)
	c.observer.Observe(routing.Event{Op: routing.OpNearest, Latency: time.Since(start), Err: err})
	if err != nil {
		return nil, err
	}

	if len(resp.Waypoints) == 0 {
		return nil, routing.ErrNoSegment
	}

	out := make([]routing.SnappedPoint, len(resp.Waypoints))
	for i, w := range resp.Waypoints {
		out[i] = routing.SnappedPoint{
			LatLng:   routing.LatLng{Lat: w.Location[1], Lng: w.Location[0]},
			Distance: w.Distance,
			Name:     w.Name,
		}
	}
	return out, nil
}
