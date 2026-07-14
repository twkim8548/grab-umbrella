package handler

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/twkim8548/grab-umbrella/server/internal/store"
	"github.com/twkim8548/grab-umbrella/server/internal/weather"
)

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
