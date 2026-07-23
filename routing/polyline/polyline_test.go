package polyline

import (
	"math"
	"testing"
)

// TestDecode_GoogleReferenceExample uses the worked example from Google's
// published polyline algorithm documentation, at precision 5.
func TestDecode_GoogleReferenceExample(t *testing.T) {
	const encoded = "_p~iF~ps|U_ulLnnqC_mqNvxq`@"
	want := []LatLng{
		{Lat: 38.5, Lng: -120.2},
		{Lat: 40.7, Lng: -120.95},
		{Lat: 43.252, Lng: -126.453},
	}

	got, err := Decode(encoded, 5)
	if err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d points, want %d", len(got), len(want))
	}
	for i := range want {
		if !almostEqual(got[i].Lat, want[i].Lat) || !almostEqual(got[i].Lng, want[i].Lng) {
			t.Errorf("point %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestEncode_GoogleReferenceExample(t *testing.T) {
	points := []LatLng{
		{Lat: 38.5, Lng: -120.2},
		{Lat: 40.7, Lng: -120.95},
		{Lat: 43.252, Lng: -126.453},
	}
	want := "_p~iF~ps|U_ulLnnqC_mqNvxq`@"

	got := Encode(points, 5)
	if got != want {
		t.Errorf("Encode() = %q, want %q", got, want)
	}
}

func TestRoundTrip_Precision6(t *testing.T) {
	// OSRM's default precision. Nairobi CBD to JKIA, roughly.
	points := []LatLng{
		{Lat: -1.286389, Lng: 36.817223},
		{Lat: -1.319167, Lng: 36.850278},
		{Lat: -1.363389, Lng: 36.927811},
	}

	encoded := Encode(points, 6)
	decoded, err := Decode(encoded, 6)
	if err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}
	if len(decoded) != len(points) {
		t.Fatalf("got %d points, want %d", len(decoded), len(points))
	}
	for i := range points {
		if math.Abs(decoded[i].Lat-points[i].Lat) > 1e-6 {
			t.Errorf("point %d lat: got %v, want %v", i, decoded[i].Lat, points[i].Lat)
		}
		if math.Abs(decoded[i].Lng-points[i].Lng) > 1e-6 {
			t.Errorf("point %d lng: got %v, want %v", i, decoded[i].Lng, points[i].Lng)
		}
	}
}

func TestDecode_Empty(t *testing.T) {
	got, err := Decode("", 5)
	if err != nil {
		t.Fatalf("Decode(\"\") returned error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("Decode(\"\") = %v, want empty", got)
	}
}

func TestDecode_Truncated(t *testing.T) {
	// A single valid latitude token with no matching longitude.
	if _, err := Decode("_p~iF", 5); err == nil {
		t.Error("Decode with missing longitude should return an error")
	}
}

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-5
}
