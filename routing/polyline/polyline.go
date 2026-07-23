// Package polyline implements the Google/OSRM encoded polyline algorithm
// at arbitrary coordinate precision. OSRM encodes route geometry as
// polyline6 (1e6 precision) by default; other precisions (e.g. the
// standard polyline5) are supported for interoperability with other
// engines or tools.
package polyline

import (
	"fmt"
	"math"
	"strings"
)

// LatLng mirrors routing.LatLng without importing the parent package, so
// this package stays a leaf with zero internal dependencies.
type LatLng struct {
	Lat float64
	Lng float64
}

// Encode encodes points into a polyline string at the given precision
// (number of decimal digits preserved; OSRM uses 6, Google Maps uses 5).
func Encode(points []LatLng, precision int) string {
	factor := math.Pow(10, float64(precision))

	var b strings.Builder
	var prevLat, prevLng int64

	for _, p := range points {
		lat := round(p.Lat * factor)
		lng := round(p.Lng * factor)

		encodeValue(&b, lat-prevLat)
		encodeValue(&b, lng-prevLng)

		prevLat, prevLng = lat, lng
	}

	return b.String()
}

// Decode decodes an encoded polyline string at the given precision back
// into coordinates.
func Decode(encoded string, precision int) ([]LatLng, error) {
	factor := math.Pow(10, float64(precision))

	var points []LatLng
	var lat, lng int64
	i := 0

	for i < len(encoded) {
		dlat, n, err := decodeValue(encoded, i)
		if err != nil {
			return nil, fmt.Errorf("polyline: decode latitude at byte %d: %w", i, err)
		}
		i = n
		lat += dlat

		if i >= len(encoded) {
			return nil, fmt.Errorf("polyline: truncated encoding, missing longitude for point %d", len(points))
		}

		dlng, n, err := decodeValue(encoded, i)
		if err != nil {
			return nil, fmt.Errorf("polyline: decode longitude at byte %d: %w", i, err)
		}
		i = n
		lng += dlng

		points = append(points, LatLng{
			Lat: float64(lat) / factor,
			Lng: float64(lng) / factor,
		})
	}

	return points, nil
}

func round(v float64) int64 {
	if v >= 0 {
		return int64(v + 0.5)
	}
	return int64(v - 0.5)
}

func encodeValue(b *strings.Builder, v int64) {
	shifted := v << 1
	if v < 0 {
		shifted = ^shifted
	}
	for shifted >= 0x20 {
		b.WriteByte(byte((0x20 | (shifted & 0x1f)) + 63)) //nolint:gosec // masked to 5 bits by &0x1f, always in [63,95]
		shifted >>= 5
	}
	b.WriteByte(byte(shifted + 63)) //nolint:gosec // loop exit invariant: shifted < 0x20 here, so result is always in [63,94]
}

func decodeValue(s string, start int) (value int64, next int, err error) {
	var result int64
	var shift uint
	i := start

	for {
		if i >= len(s) {
			return 0, i, fmt.Errorf("unexpected end of string")
		}
		b := int64(s[i]) - 63
		i++
		if b < 0 {
			return 0, i, fmt.Errorf("invalid byte 0x%x", s[i-1])
		}
		result |= (b & 0x1f) << shift
		shift += 5
		if b < 0x20 {
			break
		}
	}

	if result&1 != 0 {
		return ^(result >> 1), i, nil
	}
	return result >> 1, i, nil
}
