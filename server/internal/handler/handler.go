// Package handler 는 HTTP 핸들러를 모은다. spec §3 API 3개.
package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/twkim8548/grab-umbrella/server/internal/grid"
	"github.com/twkim8548/grab-umbrella/server/internal/store"
	"github.com/twkim8548/grab-umbrella/server/internal/weather"
)

type Handler struct {
	Store   *store.Store
	Weather *weather.Client
}

// syncRequest — POST /sync 입력. 앱이 위경도로 올리면 서버가 격자로 변환.
type syncRequest struct {
	PushToken    string  `json:"push_token"`
	HomeLat      float64 `json:"home_lat"`
	HomeLng      float64 `json:"home_lng"`
	WorkLat      float64 `json:"work_lat"`
	WorkLng      float64 `json:"work_lng"`
	CommuteStart string  `json:"commute_start"` // "0900"
	CommuteEnd   string  `json:"commute_end"`   // "1800"
}

// Sync 는 POST /sync. 위경도→격자 변환 후 devices upsert. spec §3.
func (h *Handler) Sync(w http.ResponseWriter, r *http.Request) {
	var req syncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.PushToken == "" {
		http.Error(w, "push_token required", http.StatusBadRequest)
		return
	}
	hnx, hny := grid.ToGrid(req.HomeLat, req.HomeLng)
	wnx, wny := grid.ToGrid(req.WorkLat, req.WorkLng)

	d := store.Device{
		PushToken:    req.PushToken,
		HomeNx:       hnx,
		HomeNy:       hny,
		WorkNx:       wnx,
		WorkNy:       wny,
		CommuteStart: req.CommuteStart,
		CommuteEnd:   req.CommuteEnd,
	}
	if err := h.Store.Upsert(r.Context(), d); err != nil {
		http.Error(w, "upsert failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// forecastResponse 는 GET /forecast 응답이다. 앱 ForecastResponse 와 일치.
// 데이터가 없는 슬롯은 null 로 내려 앱이 "불러오는 중"을 graceful 하게 표시하게 한다.
type forecastResponse struct {
	Morning *weather.SlotCard `json:"morning"`
	Evening *weather.SlotCard `json:"evening"`
}

// Forecast 는 GET /forecast. 출근/퇴근 카드용 가공 데이터 + 시간별 흐름. spec §3·§7.1.
func (h *Handler) Forecast(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("push_token")
	if token == "" {
		http.Error(w, "push_token required", http.StatusBadRequest)
		return
	}

	dev, err := h.Store.GetByToken(r.Context(), token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "device not found", http.StatusNotFound)
			return
		}
		log.Printf("forecast: GetByToken: %v", err)
		http.Error(w, "lookup failed", http.StatusInternalServerError)
		return
	}

	now := time.Now()
	// 출근/퇴근 각 슬롯의 fcstDate(오늘/내일) + 정시 fcstTime 산출.
	mDate, mTime := weather.SlotDateTime(now, dev.CommuteStart)
	eDate, eTime := weather.SlotDateTime(now, dev.CommuteEnd)

	// 집 격자 = 출근 카드, 회사 격자 = 퇴근 카드. 같은 격자면 캐시로 호출 공유된다.
	homeItems, err := h.Weather.VilageForecast(r.Context(), dev.HomeNx, dev.HomeNy)
	if err != nil {
		log.Printf("forecast: VilageForecast(home %d,%d): %v", dev.HomeNx, dev.HomeNy, err)
		http.Error(w, "weather upstream failed", http.StatusBadGateway)
		return
	}
	workItems, err := h.Weather.VilageForecast(r.Context(), dev.WorkNx, dev.WorkNy)
	if err != nil {
		log.Printf("forecast: VilageForecast(work %d,%d): %v", dev.WorkNx, dev.WorkNy, err)
		http.Error(w, "weather upstream failed", http.StatusBadGateway)
		return
	}

	mBefore, mAfter := weather.MorningWindow()
	eBefore, eAfter := weather.EveningWindow()

	var resp forecastResponse
	if card, ok := weather.BuildSlotCard(homeItems, mDate, mTime, mBefore, mAfter); ok {
		resp.Morning = &card
	}
	if card, ok := weather.BuildSlotCard(workItems, eDate, eTime, eBefore, eAfter); ok {
		resp.Evening = &card
	}

	writeJSON(w, resp)
}

// ForecastNow 는 GET /forecast/now (선택). 초단기실황 "지금 바깥". spec §3.
func (h *Handler) ForecastNow(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
