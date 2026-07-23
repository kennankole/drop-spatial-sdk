package osrm

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kennankole/drop-spatial-sdk/routing"
)

func sampleTrace() []routing.TracePoint {
	base := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	return []routing.TracePoint{
		{LatLng: routing.LatLng{Lat: -1.286389, Lng: 36.817223}, Timestamp: base, Accuracy: 5},
		{LatLng: routing.LatLng{Lat: -1.288000, Lng: 36.820000}, Timestamp: base.Add(3 * time.Second), Accuracy: 8},
		{LatLng: routing.LatLng{Lat: -1.290000, Lng: 36.822000}, Timestamp: base.Add(6 * time.Second), Accuracy: 5},
	}
}

func TestClient_Match_Success(t *testing.T) {
	srv := serveFixture(t, "testdata/match_ok.json", http.StatusOK)
	defer srv.Close()

	c := New(srv.URL)
	result, err := c.Match(context.Background(), sampleTrace())
	if err != nil {
		t.Fatalf("Match returned error: %v", err)
	}
	if result.Confidence != 0.87 {
		t.Errorf("Confidence = %v, want 0.87", result.Confidence)
	}
	if len(result.Geometry) != 3 {
		t.Errorf("Geometry has %d points, want 3", len(result.Geometry))
	}
	if len(result.Legs) != 2 {
		t.Errorf("Legs has %d entries, want 2", len(result.Legs))
	}
}

func TestClient_Match_RequestShape(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		body, _ := os.ReadFile("testdata/match_ok.json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.Match(context.Background(), sampleTrace())
	if err != nil {
		t.Fatalf("Match returned error: %v", err)
	}
	if !strings.Contains(gotQuery, "timestamps=") {
		t.Errorf("query = %q, want timestamps param", gotQuery)
	}
	if !strings.Contains(gotQuery, "radiuses=5.0%3B8.0%3B5.0") {
		t.Errorf("query = %q, want per-point radiuses from Accuracy", gotQuery)
	}
}

func TestClient_Match_TooFewPoints(t *testing.T) {
	c := New("http://unused.invalid")
	_, err := c.Match(context.Background(), sampleTrace()[:1])
	if !errors.Is(err, routing.ErrInvalidInput) {
		t.Errorf("Match error = %v, want ErrInvalidInput", err)
	}
}

func TestClient_Match_RadiusesLengthMismatch(t *testing.T) {
	c := New("http://unused.invalid")
	_, err := c.Match(context.Background(), sampleTrace(), routing.WithRadiuses([]float64{5, 5}))
	if !errors.Is(err, routing.ErrInvalidInput) {
		t.Errorf("Match error = %v, want ErrInvalidInput", err)
	}
}

func TestClient_Match_NoMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":"Ok","matchings":[]}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.Match(context.Background(), sampleTrace())
	if !errors.Is(err, routing.ErrNoRoute) {
		t.Errorf("Match error = %v, want ErrNoRoute for empty matchings", err)
	}
}
