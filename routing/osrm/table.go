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

// osrmTableResponse mirrors the relevant subset of OSRM's /table response:
// https://project-osrm.org/docs/v5.24.0/api/#table-service
//
// Durations/Distances entries are *float64 because OSRM returns JSON null
// for pairs with no route between them.
type osrmTableResponse struct {
	Code      string       `json:"code"`
	Durations [][]*float64 `json:"durations"`
	Distances [][]*float64 `json:"distances,omitempty"`
}

// Table implements routing.Matrix.
//
// Rider Matching Machine's ranking step is the primary consumer: sources
// is typically the rider candidate pool, destinations the single store
// location (or vice versa), and the resulting Durations give road-network
// travel time instead of the straight-line distance the ranking algorithm
// uses today.
func (c *Client) Table(ctx context.Context, sources, destinations []routing.LatLng, opts ...routing.TableOption) (*routing.DurationMatrix, error) {
	if len(sources) == 0 || len(destinations) == 0 {
		return nil, fmt.Errorf("%w: sources and destinations must be non-empty", routing.ErrInvalidInput)
	}
	for _, p := range sources {
		if !p.Valid() {
			return nil, routing.ErrInvalidInput
		}
	}
	for _, p := range destinations {
		if !p.Valid() {
			return nil, routing.ErrInvalidInput
		}
	}

	o := routing.ResolveTableOptions(opts...)

	// OSRM /table takes one combined coordinate list plus index sets for
	// which entries act as sources and which as destinations.
	coords := make([]string, 0, len(sources)+len(destinations))
	sourceIdx := make([]string, len(sources))
	destIdx := make([]string, len(destinations))

	for i, p := range sources {
		coords = append(coords, formatCoord(p))
		sourceIdx[i] = strconv.Itoa(i)
	}
	offset := len(sources)
	for i, p := range destinations {
		coords = append(coords, formatCoord(p))
		destIdx[i] = strconv.Itoa(offset + i)
	}

	q := url.Values{}
	q.Set("sources", strings.Join(sourceIdx, ";"))
	q.Set("destinations", strings.Join(destIdx, ";"))
	if o.WithDistances {
		q.Set("annotations", "duration,distance")
	}

	reqURL := fmt.Sprintf("%s/table/v1/%s/%s?%s",
		strings.TrimRight(c.baseURL, "/"),
		c.profile,
		strings.Join(coords, ";"),
		q.Encode(),
	)

	start := time.Now()
	var resp osrmTableResponse
	err := c.doRequest(ctx, reqURL, &resp)
	c.observer.Observe(routing.Event{Op: routing.OpTable, Latency: time.Since(start), Err: err})
	if err != nil {
		return nil, err
	}

	return toDurationMatrix(sources, destinations, resp), nil
}

func toDurationMatrix(sources, destinations []routing.LatLng, resp osrmTableResponse) *routing.DurationMatrix {
	durations := make([][]time.Duration, len(resp.Durations))
	for i, row := range resp.Durations {
		durations[i] = make([]time.Duration, len(row))
		for j, v := range row {
			if v == nil {
				durations[i][j] = -1
				continue
			}
			durations[i][j] = secondsToDuration(*v)
		}
	}

	var distances [][]float64
	if resp.Distances != nil {
		distances = make([][]float64, len(resp.Distances))
		for i, row := range resp.Distances {
			distances[i] = make([]float64, len(row))
			for j, v := range row {
				if v == nil {
					distances[i][j] = -1
					continue
				}
				distances[i][j] = *v
			}
		}
	}

	return &routing.DurationMatrix{
		Sources:      sources,
		Destinations: destinations,
		Durations:    durations,
		Distances:    distances,
	}
}
