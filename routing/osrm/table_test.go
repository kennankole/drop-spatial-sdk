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

func TestClient_Table_Success(t *testing.T) {
	srv := serveFixture(t, "testdata/table_ok.json", http.StatusOK)
	defer srv.Close()

	sources := []routing.LatLng{{Lat: -1.28, Lng: 36.81}, {Lat: -1.29, Lng: 36.82}}
	destinations := []routing.LatLng{{Lat: -1.30, Lng: 36.83}, {Lat: -1.31, Lng: 36.84}, {Lat: -1.32, Lng: 36.85}}

	c := New(srv.URL)
	m, err := c.Table(context.Background(), sources, destinations, routing.WithDistances(true))
	if err != nil {
		t.Fatalf("Table returned error: %v", err)
	}

	if len(m.Durations) != 2 || len(m.Durations[0]) != 3 {
		t.Fatalf("Durations shape = %dx%d, want 2x3", len(m.Durations), len(m.Durations[0]))
	}
	if m.Durations[0][0] != 70300*time.Millisecond {
		t.Errorf("Durations[0][0] = %v, want 70.3s", m.Durations[0][0])
	}
	if m.Durations[0][2] != -1 {
		t.Errorf("Durations[0][2] (null pair) = %v, want -1", m.Durations[0][2])
	}
	if m.Distances == nil {
		t.Fatal("Distances is nil, want populated (WithDistances(true) was set)")
	}
	if m.Distances[0][2] != -1 {
		t.Errorf("Distances[0][2] (null pair) = %v, want -1", m.Distances[0][2])
	}
	if len(m.Sources) != 2 || len(m.Destinations) != 3 {
		t.Errorf("Sources/Destinations echo = %d/%d, want 2/3", len(m.Sources), len(m.Destinations))
	}
}

func TestClient_Table_WithoutDistances_OmitsAnnotation(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		body, _ := os.ReadFile("testdata/table_ok.json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.Table(context.Background(), []routing.LatLng{{Lat: -1.28, Lng: 36.81}}, []routing.LatLng{{Lat: -1.30, Lng: 36.83}})
	if err != nil {
		t.Fatalf("Table returned error: %v", err)
	}
	if strings.Contains(gotQuery, "annotations") {
		t.Errorf("query = %q, want no annotations param when WithDistances not set", gotQuery)
	}
	if !strings.Contains(gotQuery, "sources=0") || !strings.Contains(gotQuery, "destinations=1") {
		t.Errorf("query = %q, want sources=0 and destinations=1 (index-based)", gotQuery)
	}
}

func TestClient_Table_EmptyInput(t *testing.T) {
	c := New("http://unused.invalid")
	_, err := c.Table(context.Background(), nil, []routing.LatLng{{Lat: 0, Lng: 0}})
	if !errors.Is(err, routing.ErrInvalidInput) {
		t.Errorf("Table error = %v, want ErrInvalidInput", err)
	}
}
