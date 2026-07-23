package osrm

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kennankole/drop-spatial-sdk/routing"
)

func TestClient_Nearest_Success(t *testing.T) {
	srv := serveFixture(t, "testdata/nearest_ok.json", http.StatusOK)
	defer srv.Close()

	c := New(srv.URL)
	points, err := c.Nearest(context.Background(), nairobiCBD, 2)
	if err != nil {
		t.Fatalf("Nearest returned error: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("got %d points, want 2", len(points))
	}
	if points[0].Name != "Moi Avenue" {
		t.Errorf("Name = %q, want %q", points[0].Name, "Moi Avenue")
	}
	if points[0].Distance != 4.2 {
		t.Errorf("Distance = %v, want 4.2", points[0].Distance)
	}
}

func TestClient_Nearest_InvalidN(t *testing.T) {
	c := New("http://unused.invalid")
	_, err := c.Nearest(context.Background(), nairobiCBD, 0)
	if !errors.Is(err, routing.ErrInvalidInput) {
		t.Errorf("Nearest error = %v, want ErrInvalidInput", err)
	}
}

func TestClient_Nearest_NoSegment(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":"Ok","waypoints":[]}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.Nearest(context.Background(), nairobiCBD, 1)
	if !errors.Is(err, routing.ErrNoSegment) {
		t.Errorf("Nearest error = %v, want ErrNoSegment for empty waypoints", err)
	}
}
