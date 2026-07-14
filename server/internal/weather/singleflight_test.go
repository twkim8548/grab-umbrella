package weather

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestVilageForecastCoalescesConcurrentMisses(t *testing.T) {
	var calls atomic.Int32
	c := New("key", "https://weather.test")
	c.httpClient = &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		calls.Add(1)
		time.Sleep(40 * time.Millisecond)
		return testHTTPResponse(forecastTestResponse), nil
	})}
	c.now = func() time.Time { return time.Date(2026, 6, 23, 8, 20, 0, 0, kst) }
	const n = 20
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			<-start
			if _, err := c.VilageForecast(context.Background(), 60, 127); err != nil {
				t.Errorf("VilageForecast: %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()
	if got := calls.Load(); got != 1 {
		t.Fatalf("upstream calls = %d, want 1", got)
	}
}

func TestSharedFetchCallerCancellationDoesNotCancelOtherWaiter(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	c := New("key", "https://weather.test")
	c.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		once.Do(func() { close(started) })
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-release:
			return testHTTPResponse(forecastTestResponse), nil
		}
	})}
	c.now = func() time.Time { return time.Date(2026, 6, 23, 8, 20, 0, 0, kst) }
	firstCtx, cancelFirst := context.WithCancel(context.Background())
	firstDone := make(chan error, 1)
	go func() {
		_, err := c.VilageForecast(firstCtx, 60, 127)
		firstDone <- err
	}()
	<-started

	secondDone := make(chan error, 1)
	go func() {
		_, err := c.VilageForecast(context.Background(), 60, 127)
		secondDone <- err
	}()
	cancelFirst()
	if err := <-firstDone; err != context.Canceled {
		t.Fatalf("first caller error = %v, want context.Canceled", err)
	}
	close(release)
	if err := <-secondDone; err != nil {
		t.Fatalf("second waiter was affected by first cancellation: %v", err)
	}
}

func TestCanceledCallerDoesNotStartSharedFetch(t *testing.T) {
	var calls atomic.Int32
	c := New("key", "https://weather.test")
	c.httpClient = &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		calls.Add(1)
		return testHTTPResponse(forecastTestResponse), nil
	})}
	c.now = func() time.Time { return time.Date(2026, 6, 23, 8, 20, 0, 0, kst) }
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := c.VilageForecast(ctx, 60, 127); err != context.Canceled {
		t.Fatalf("VilageForecast error = %v, want context.Canceled", err)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("upstream calls = %d, want 0", got)
	}
}

const forecastTestResponse = `{"response":{"header":{"resultCode":"00","resultMsg":"NORMAL_SERVICE"},"body":{"items":{"item":[{"category":"TMP","fcstDate":"20260623","fcstTime":"0900","fcstValue":"23","nx":60,"ny":127}]}}}}`

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func testHTTPResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}
