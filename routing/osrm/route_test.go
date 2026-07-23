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

func serveFixture(t *testing.T, path string, status int) *httptest.Server {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
}

var (
	nairobiCBD = routing.LatLng{Lat: -1.286389, Lng: 36.817223}
	nairobiTMB = routing.LatLng{Lat: -1.290000, Lng: 36.822000}
)

func TestClient_Route_Success(t *testing.T) {
	srv := serveFixture(t, "testdata/route_ok.json", http.StatusOK)
	defer srv.Close()

	c := New(srv.URL)
	route, err := c.Route(context.Background(), nairobiCBD, nairobiTMB)
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}

	if route.Distance != 1250.4 {
		t.Errorf("Distance = %v, want 1250.4", route.Distance)
	}
	wantDuration := time.Duration(142.7 * float64(time.Second))
	if route.Duration != wantDuration {
		t.Errorf("Duration = %v, want %v", route.Duration, wantDuration)
	}
	if len(route.Geometry) != 3 {
		t.Errorf("Geometry has %d points, want 3", len(route.Geometry))
	}
	if len(route.Legs) != 1 {
		t.Fatalf("Legs has %d entries, want 1", len(route.Legs))
	}
	if len(route.Legs[0].Steps) != 2 {
		t.Fatalf("Steps has %d entries, want 2", len(route.Legs[0].Steps))
	}
	if route.Legs[0].Steps[0].Name != "Moi Avenue" {
		t.Errorf("Steps[0].Name = %q, want %q", route.Legs[0].Steps[0].Name, "Moi Avenue")
	}
	if route.Legs[0].Steps[1].ManeuverType != "arrive" {
		t.Errorf("Steps[1].ManeuverType = %q, want %q", route.Legs[0].Steps[1].ManeuverType, "arrive")
	}
}

func TestClient_Route_RequestShape(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path + "?" + r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		body, _ := os.ReadFile("testdata/route_ok.json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.Route(context.Background(), nairobiCBD, nairobiTMB, routing.WithSteps(true))
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}

	if !strings.HasPrefix(gotPath, "/route/v1/driving/36.817223,-1.286389;36.822000,-1.290000") {
		t.Errorf("request path = %q, want coordinates in lng,lat order", gotPath)
	}
	if !strings.Contains(gotPath, "steps=true") {
		t.Errorf("request path = %q, want steps=true", gotPath)
	}
	if !strings.Contains(gotPath, "geometries=polyline6") {
		t.Errorf("request path = %q, want geometries=polyline6", gotPath)
	}
}

func TestClient_Route_NoRoute(t *testing.T) {
	srv := serveFixture(t, "testdata/route_no_route.json", http.StatusBadRequest)
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.Route(context.Background(), nairobiCBD, nairobiTMB)
	if !errors.Is(err, routing.ErrNoRoute) {
		t.Errorf("Route error = %v, want ErrNoRoute", err)
	}
}

func TestClient_Route_InvalidInput_NoNetworkCall(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(srv.URL)
	invalid := routing.LatLng{Lat: 999, Lng: 0}
	_, err := c.Route(context.Background(), invalid, nairobiTMB)
	if !errors.Is(err, routing.ErrInvalidInput) {
		t.Errorf("Route error = %v, want ErrInvalidInput", err)
	}
	if called {
		t.Error("server was called despite invalid input; validation should short-circuit before the network call")
	}
}

func TestClient_Route_ServerError_MapsToUnavailable(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := New(srv.URL, WithRetries(2))
	_, err := c.Route(context.Background(), nairobiCBD, nairobiTMB)
	if !errors.Is(err, routing.ErrUnavailable) {
		t.Errorf("Route error = %v, want ErrUnavailable", err)
	}
	if attempts != 3 {
		t.Errorf("server received %d attempts, want 3 (1 initial + 2 retries)", attempts)
	}
}

func TestClient_Route_ServerError_RetrySucceeds(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		body, _ := os.ReadFile("testdata/route_ok.json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	c := New(srv.URL, WithRetries(2))
	route, err := c.Route(context.Background(), nairobiCBD, nairobiTMB)
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}
	if route.Distance != 1250.4 {
		t.Errorf("Distance = %v, want 1250.4", route.Distance)
	}
	if attempts != 2 {
		t.Errorf("server received %d attempts, want 2", attempts)
	}
}

func TestClient_Route_TransportFailure_MapsToUnavailable(t *testing.T) {
	// Port 0 on loopback with no listener: connection refused.
	c := New("http://127.0.0.1:1", WithTimeout(200*time.Millisecond), WithRetries(0))
	_, err := c.Route(context.Background(), nairobiCBD, nairobiTMB)
	if !errors.Is(err, routing.ErrUnavailable) {
		t.Errorf("Route error = %v, want ErrUnavailable", err)
	}
}

func TestClient_Route_ObserverNotified(t *testing.T) {
	srv := serveFixture(t, "testdata/route_ok.json", http.StatusOK)
	defer srv.Close()

	var events []routing.Event
	obs := routing.ObserverFunc(func(e routing.Event) { events = append(events, e) })

	c := New(srv.URL, WithObserver(obs))
	_, err := c.Route(context.Background(), nairobiCBD, nairobiTMB)
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if events[0].Op != routing.OpRoute {
		t.Errorf("Op = %v, want OpRoute", events[0].Op)
	}
	if events[0].Err != nil {
		t.Errorf("Err = %v, want nil", events[0].Err)
	}
}
