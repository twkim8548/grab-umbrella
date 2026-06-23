package geocode

import "testing"

func TestParseGeocode(t *testing.T) {
	const sample = `{
		"documents": [
			{"address_name": "서울 중구 세종대로 110", "x": "126.977829174031", "y": "37.5663174209601"}
		],
		"meta": {"total_count": 1}
	}`

	lat, lng, err := parseGeocode([]byte(sample))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lat != 37.5663174209601 {
		t.Errorf("lat = %v, want 37.5663174209601", lat)
	}
	if lng != 126.977829174031 {
		t.Errorf("lng = %v, want 126.977829174031", lng)
	}
}

func TestParseGeocodeEmptyDocuments(t *testing.T) {
	const sample = `{"documents": [], "meta": {"total_count": 0}}`

	_, _, err := parseGeocode([]byte(sample))
	if err == nil {
		t.Fatal("expected error for empty documents, got nil")
	}
}

func TestParseGeocodeInvalidJSON(t *testing.T) {
	_, _, err := parseGeocode([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestGeocodeEmptyKey(t *testing.T) {
	c := New("")
	_, _, err := c.Geocode(nil, "서울 중구 세종대로 110")
	if err == nil {
		t.Fatal("expected error for empty key, got nil")
	}
}
