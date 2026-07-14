package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/twkim8548/grab-umbrella/server/internal/grid"
	"github.com/twkim8548/grab-umbrella/server/internal/store"
	"github.com/twkim8548/grab-umbrella/server/internal/weather"
)

type fakeDeviceSyncStore struct {
	existing    *store.Device
	getErr      error
	upsertErr   error
	getCalls    int
	upsertCalls int
	upserted    *store.Device
}

func (s *fakeDeviceSyncStore) GetByToken(context.Context, string) (*store.Device, error) {
	s.getCalls++
	return s.existing, s.getErr
}

func (s *fakeDeviceSyncStore) Upsert(_ context.Context, d store.Device) error {
	s.upsertCalls++
	s.upserted = &d
	return s.upsertErr
}

type fakeGeocodeProvider struct {
	points map[string][2]float64
	calls  []string
}

func (g *fakeGeocodeProvider) Geocode(_ context.Context, address string) (float64, float64, error) {
	g.calls = append(g.calls, address)
	point, ok := g.points[address]
	if !ok {
		return 0, 0, errors.New("address not found")
	}
	return point[0], point[1], nil
}

func TestSyncReusesUnchangedAddressGrids(t *testing.T) {
	homePoint := [2]float64{37.5665, 126.9780}
	workPoint := [2]float64{37.4979, 127.0276}
	newWorkPoint := [2]float64{37.4000, 127.1000}
	newHomePoint := [2]float64{37.6000, 126.9000}
	homeNx, homeNy := grid.ToGrid(homePoint[0], homePoint[1])
	workNx, workNy := grid.ToGrid(workPoint[0], workPoint[1])
	newWorkNx, newWorkNy := grid.ToGrid(newWorkPoint[0], newWorkPoint[1])
	newHomeNx, newHomeNy := grid.ToGrid(newHomePoint[0], newHomePoint[1])

	tests := []struct {
		name         string
		existing     *store.Device
		getErr       error
		homeAddress  string
		workAddress  string
		wantCalls    []string
		wantHomeGrid [2]int
		wantWorkGrid [2]int
	}{
		{
			name:         "new device geocodes both addresses",
			getErr:       pgx.ErrNoRows,
			homeAddress:  "집 주소",
			workAddress:  "회사 주소",
			wantCalls:    []string{"집 주소", "회사 주소"},
			wantHomeGrid: [2]int{homeNx, homeNy},
			wantWorkGrid: [2]int{workNx, workNy},
		},
		{
			name: "unchanged addresses reuse both grids",
			existing: &store.Device{
				HomeAddress: "  집 주소 ", HomeNx: 11, HomeNy: 22,
				WorkAddress: " 회사 주소  ", WorkNx: 33, WorkNy: 44,
			},
			homeAddress:  "집 주소",
			workAddress:  "회사 주소",
			wantHomeGrid: [2]int{11, 22},
			wantWorkGrid: [2]int{33, 44},
		},
		{
			name: "only changed work address is geocoded",
			existing: &store.Device{
				HomeAddress: "집 주소", HomeNx: 11, HomeNy: 22,
				WorkAddress: "회사 주소", WorkNx: 33, WorkNy: 44,
			},
			homeAddress:  "집 주소",
			workAddress:  "새 회사 주소",
			wantCalls:    []string{"새 회사 주소"},
			wantHomeGrid: [2]int{11, 22},
			wantWorkGrid: [2]int{newWorkNx, newWorkNy},
		},
		{
			name: "only changed home address is geocoded",
			existing: &store.Device{
				HomeAddress: "집 주소", HomeNx: 11, HomeNy: 22,
				WorkAddress: "회사 주소", WorkNx: 33, WorkNy: 44,
			},
			homeAddress:  "새 집 주소",
			workAddress:  "회사 주소",
			wantCalls:    []string{"새 집 주소"},
			wantHomeGrid: [2]int{newHomeNx, newHomeNy},
			wantWorkGrid: [2]int{33, 44},
		},
		{
			name: "legacy empty address is geocoded again",
			existing: &store.Device{
				HomeAddress: "", HomeNx: 11, HomeNy: 22,
				WorkAddress: "회사 주소", WorkNx: 33, WorkNy: 44,
			},
			homeAddress:  "집 주소",
			workAddress:  "회사 주소",
			wantCalls:    []string{"집 주소"},
			wantHomeGrid: [2]int{homeNx, homeNy},
			wantWorkGrid: [2]int{33, 44},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeStore := &fakeDeviceSyncStore{existing: tt.existing, getErr: tt.getErr}
			fakeGeocoder := &fakeGeocodeProvider{points: map[string][2]float64{
				"집 주소":    homePoint,
				"새 집 주소":  newHomePoint,
				"회사 주소":   workPoint,
				"새 회사 주소": newWorkPoint,
			}}
			h := &Handler{syncStore: fakeStore, geocoder: fakeGeocoder}
			body := `{
				"push_token":"token",
				"home_address":"  ` + tt.homeAddress + `  ",
				"work_address":"  ` + tt.workAddress + `  ",
				"commute_start":"0900",
				"commute_end":"1800"
			}`
			req := httptest.NewRequest(http.MethodPost, "/sync", strings.NewReader(body))
			w := httptest.NewRecorder()

			h.Sync(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("Sync() status = %d, want %d; body=%q", w.Code, http.StatusOK, w.Body.String())
			}
			if strings.Join(fakeGeocoder.calls, "|") != strings.Join(tt.wantCalls, "|") {
				t.Fatalf("geocode calls = %v, want %v", fakeGeocoder.calls, tt.wantCalls)
			}
			if fakeStore.upserted == nil {
				t.Fatal("Sync() did not upsert device")
			}
			got := fakeStore.upserted
			if got.HomeNx != tt.wantHomeGrid[0] || got.HomeNy != tt.wantHomeGrid[1] {
				t.Errorf("home grid = (%d,%d), want (%d,%d)", got.HomeNx, got.HomeNy, tt.wantHomeGrid[0], tt.wantHomeGrid[1])
			}
			if got.WorkNx != tt.wantWorkGrid[0] || got.WorkNy != tt.wantWorkGrid[1] {
				t.Errorf("work grid = (%d,%d), want (%d,%d)", got.WorkNx, got.WorkNy, tt.wantWorkGrid[0], tt.wantWorkGrid[1])
			}
			if got.HomeAddress != tt.homeAddress || got.WorkAddress != tt.workAddress {
				t.Errorf("stored addresses = (%q,%q), want (%q,%q)", got.HomeAddress, got.WorkAddress, tt.homeAddress, tt.workAddress)
			}
		})
	}
}

func TestSyncFailurePaths(t *testing.T) {
	tests := []struct {
		name             string
		getErr           error
		upsertErr        error
		points           map[string][2]float64
		wantStatus       int
		wantGeocodeCalls []string
		wantUpsertCalls  int
	}{
		{
			name:            "lookup error stops before geocoding",
			getErr:          errors.New("database unavailable"),
			points:          map[string][2]float64{"집 주소": {37.5, 127.0}, "회사 주소": {37.4, 127.1}},
			wantStatus:      http.StatusInternalServerError,
			wantUpsertCalls: 0,
		},
		{
			name:             "home geocode error stops before upsert",
			getErr:           pgx.ErrNoRows,
			points:           map[string][2]float64{"회사 주소": {37.4, 127.1}},
			wantStatus:       http.StatusUnprocessableEntity,
			wantGeocodeCalls: []string{"집 주소"},
		},
		{
			name:             "work geocode error stops before upsert",
			getErr:           pgx.ErrNoRows,
			points:           map[string][2]float64{"집 주소": {37.5, 127.0}},
			wantStatus:       http.StatusUnprocessableEntity,
			wantGeocodeCalls: []string{"집 주소", "회사 주소"},
		},
		{
			name:             "upsert error returns server error",
			getErr:           pgx.ErrNoRows,
			upsertErr:        errors.New("database unavailable"),
			points:           map[string][2]float64{"집 주소": {37.5, 127.0}, "회사 주소": {37.4, 127.1}},
			wantStatus:       http.StatusInternalServerError,
			wantGeocodeCalls: []string{"집 주소", "회사 주소"},
			wantUpsertCalls:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeStore := &fakeDeviceSyncStore{getErr: tt.getErr, upsertErr: tt.upsertErr}
			fakeGeocoder := &fakeGeocodeProvider{points: tt.points}
			h := &Handler{syncStore: fakeStore, geocoder: fakeGeocoder}
			req := httptest.NewRequest(http.MethodPost, "/sync", strings.NewReader(`{
				"push_token":"token",
				"home_address":"집 주소",
				"work_address":"회사 주소",
				"commute_start":"0900",
				"commute_end":"1800"
			}`))
			w := httptest.NewRecorder()

			h.Sync(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Sync() status = %d, want %d; body=%q", w.Code, tt.wantStatus, w.Body.String())
			}
			if fakeStore.getCalls != 1 {
				t.Errorf("GetByToken calls = %d, want 1", fakeStore.getCalls)
			}
			if strings.Join(fakeGeocoder.calls, "|") != strings.Join(tt.wantGeocodeCalls, "|") {
				t.Errorf("geocode calls = %v, want %v", fakeGeocoder.calls, tt.wantGeocodeCalls)
			}
			if fakeStore.upsertCalls != tt.wantUpsertCalls {
				t.Errorf("Upsert calls = %d, want %d", fakeStore.upsertCalls, tt.wantUpsertCalls)
			}
		})
	}
}

func TestNotificationsEnabledBackwardCompatibility(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "omitted by old client defaults to enabled",
			body: `{}`,
			want: true,
		},
		{
			name: "explicit true enables notifications",
			body: `{"notifications_enabled":true}`,
			want: true,
		},
		{
			name: "explicit false disables notifications",
			body: `{"notifications_enabled":false}`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req syncRequest
			if err := json.Unmarshal([]byte(tt.body), &req); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			if got := notificationsEnabled(req.NotificationsEnabled); got != tt.want {
				t.Fatalf("notificationsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

type delayedForecastProvider struct {
	delay    time.Duration
	failNx   int
	mu       sync.Mutex
	active   int
	maxAlive int
}

func (f *delayedForecastProvider) VilageForecast(ctx context.Context, nx, ny int) ([]weather.FcstItem, error) {
	f.mu.Lock()
	f.active++
	if f.active > f.maxAlive {
		f.maxAlive = f.active
	}
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		f.active--
		f.mu.Unlock()
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(f.delay):
	}
	if nx == f.failNx {
		return nil, errors.New("upstream failed")
	}
	temp := "11"
	if nx == 70 {
		temp = "22"
	}
	return testItems(temp), nil
}

func (f *delayedForecastProvider) UltraSrtForecast(context.Context, int, int) ([]weather.FcstItem, error) {
	return nil, errors.New("unexpected ultra request")
}

func testItems(temp string) []weather.FcstItem {
	var out []weather.FcstItem
	for _, date := range []string{"20260623", "20260624"} {
		for _, hour := range []string{"0900", "1800"} {
			out = append(out, weather.FcstItem{Category: "TMP", FcstDate: date, FcstTime: hour, FcstValue: temp})
		}
	}
	return out
}

func TestBuildForecastResponseRunsGridsInParallelAndMapsCards(t *testing.T) {
	fake := &delayedForecastProvider{delay: 60 * time.Millisecond, failNx: -1}
	h := &Handler{forecast: fake}
	dev := store.Device{HomeNx: 60, HomeNy: 127, WorkNx: 70, WorkNy: 130, CommuteStart: "0900", CommuteEnd: "1800"}
	now := time.Date(2026, 6, 23, 0, 0, 0, 0, time.FixedZone("KST", 9*60*60))

	got := h.buildForecastResponse(context.Background(), now, dev)
	if fake.maxAlive < 2 {
		t.Fatalf("max concurrent weather calls = %d, want >= 2", fake.maxAlive)
	}
	if got.Today.Morning == nil || got.Tomorrow.Morning == nil || got.Today.Morning.TempC != 11 || got.Tomorrow.Morning.TempC != 11 {
		t.Fatalf("morning cards mapped incorrectly: %+v", got)
	}
	if got.Today.Evening == nil || got.Tomorrow.Evening == nil || got.Today.Evening.TempC != 22 || got.Tomorrow.Evening.TempC != 22 {
		t.Fatalf("evening cards mapped incorrectly: %+v", got)
	}
}

func TestBuildForecastResponseKeepsSuccessfulGridOnPartialFailure(t *testing.T) {
	fake := &delayedForecastProvider{failNx: 70}
	h := &Handler{forecast: fake}
	dev := store.Device{HomeNx: 60, HomeNy: 127, WorkNx: 70, WorkNy: 130, CommuteStart: "0900", CommuteEnd: "1800"}
	now := time.Date(2026, 6, 23, 0, 0, 0, 0, time.FixedZone("KST", 9*60*60))

	got := h.buildForecastResponse(context.Background(), now, dev)
	if got.Today.Morning == nil || got.Tomorrow.Morning == nil {
		t.Fatalf("successful morning grid was lost: %+v", got)
	}
	if got.Today.Evening != nil || got.Tomorrow.Evening != nil {
		t.Fatalf("failed evening grid should be null: %+v", got)
	}
}
