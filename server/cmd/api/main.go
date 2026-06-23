// Command api 는 웹 서비스다. POST /sync, GET /forecast, GET /forecast/now. spec §3.
package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/twkim8548/grab-umbrella/server/internal/handler"
	"github.com/twkim8548/grab-umbrella/server/internal/store"
	"github.com/twkim8548/grab-umbrella/server/internal/weather"
)

func main() {
	ctx := context.Background()

	st, err := store.New(ctx, mustEnv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	// 기상청 키는 /forecast 계열에서만 필요. /sync 만 쓰는 동안엔 비어 있어도 서버가 떠야 한다.
	// (활용신청 전 단계 지원) — 키 없이 /forecast 호출 시 핸들러에서 에러 반환.
	kmaKey := os.Getenv("KMA_SERVICE_KEY")
	if kmaKey == "" {
		log.Println("warning: KMA_SERVICE_KEY not set — /forecast endpoints will fail until configured")
	}
	wc := weather.New(kmaKey, env("KMA_BASE_URL",
		"http://apis.data.go.kr/1360000/VilageFcstInfoService_2.0"))

	h := &handler.Handler{Store: st, Weather: wc}

	r := chi.NewRouter()
	r.Use(middleware.Logger, middleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	r.Post("/sync", h.Sync)
	r.Get("/forecast", h.Forecast)
	r.Get("/forecast/now", h.ForecastNow)

	port := env("PORT", "8080")
	log.Printf("api listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required env: %s", key)
	}
	return v
}
