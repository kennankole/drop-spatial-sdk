package osrm

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/kennankole/drop-spatial-sdk/routing"
	"github.com/kennankole/drop-spatial-sdk/routing/polyline"
)

// osrmRouteResponse mirrors the relevant subset of OSRM's /route response:
// https://project-osrm.org/docs/v5.24.0/api/#route-service
type osrmRouteResponse struct {
	Code   string      `json:"code"`
	Routes []osrmRoute `json:"routes"`
}

type osrmRoute struct {
	Distance float64   `json:"distance"`
	Duration float64   `json:"duration"`
	Geometry string    `json:"geometry"`
	Legs     []osrmLeg `json:"legs"`
}

type osrmLeg struct {
	Distance float64    `json:"distance"`
	Duration float64    `json:"duration"`
	Steps    []osrmStep `json:"steps"`
}

type osrmStep struct {
	Distance float64      `json:"distance"`
	Duration float64      `json:"duration"`
	Geometry string       `json:"geometry"`
	Name     string       `json:"name"`
	Maneuver osrmManeuver `json:"maneuver"`
}

type osrmManeuver struct {
	Type     string `json:"type"`
	Modifier string `json:"modifier"`
}

// Route implements routing.Router.
func (c *Client) Route(ctx context.Context, origin, destination routing.LatLng, opts ...routing.RouteOption) (*routing.Route, error) {
	if !origin.Valid() || !destination.Valid() {
		return nil, routing.ErrInvalidInput
	}

	o := routing.ResolveRouteOptions(opts...)

	q := url.Values{}
	q.Set("overview", "full")
	q.Set("geometries", "polyline6")
	q.Set("steps", strconv.FormatBool(o.Steps))
	if o.Alternatives > 0 {
		q.Set("alternatives", strconv.Itoa(o.Alternatives))
	}

	reqURL := fmt.Sprintf("%s/route/v1/%s/%s;%s?%s",
		strings.TrimRight(c.baseURL, "/"),
		c.profile,
		formatCoord(origin),
		formatCoord(destination),
		q.Encode(),
	)

	start := time.Now()
	var resp osrmRouteResponse
	err := c.doRequest(ctx, reqURL, &resp)
	c.observer.Observe(routing.Event{Op: routing.OpRoute, Latency: time.Since(start), Err: err})
	if err != nil {
		return nil, err
	}

	if len(resp.Routes) == 0 {
		return nil, routing.ErrNoRoute
	}

	return toRoute(resp.Routes[0])
}

func toRoute(r osrmRoute) (*routing.Route, error) {
	geometry, err := decodeGeometry(r.Geometry)
	if err != nil {
		return nil, err
	}

	legs := make([]routing.Leg, len(r.Legs))
	for i, l := range r.Legs {
		steps := make([]routing.Step, len(l.Steps))
		for j, s := range l.Steps {
			stepGeom, err := decodeGeometry(s.Geometry)
			if err != nil {
				return nil, err
			}
			steps[j] = routing.Step{
				Distance:     s.Distance,
				Duration:     secondsToDuration(s.Duration),
				Geometry:     stepGeom,
				Name:         s.Name,
				ManeuverType: s.Maneuver.Type,
				Modifier:     s.Maneuver.Modifier,
			}
		}
		legs[i] = routing.Leg{
			Distance: l.Distance,
			Duration: secondsToDuration(l.Duration),
			Steps:    steps,
		}
	}

	return &routing.Route{
		Distance: r.Distance,
		Duration: secondsToDuration(r.Duration),
		Geometry: geometry,
		Legs:     legs,
	}, nil
}

func decodeGeometry(encoded string) ([]routing.LatLng, error) {
	if encoded == "" {
		return nil, nil
	}
	points, err := polyline.Decode(encoded, 6)
	if err != nil {
		return nil, fmt.Errorf("osrm: decode geometry: %w", err)
	}
	out := make([]routing.LatLng, len(points))
	for i, p := range points {
		out[i] = routing.LatLng{Lat: p.Lat, Lng: p.Lng}
	}
	return out, nil
}

func secondsToDuration(s float64) time.Duration {
	return time.Duration(s * float64(time.Second))
}

// formatCoord renders a LatLng in OSRM's required lng,lat order.
func formatCoord(p routing.LatLng) string {
	return strconv.FormatFloat(p.Lng, 'f', 6, 64) + "," + strconv.FormatFloat(p.Lat, 'f', 6, 64)
}
