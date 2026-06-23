// Package handler 는 HTTP 핸들러를 모은다. spec §3 API 3개.
package handler

import (
	"encoding/json"
	"net/http"

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

// Forecast 는 GET /forecast. 출근/퇴근 카드용 가공 데이터 + 시간별 흐름. spec §3.
func (h *Handler) Forecast(w http.ResponseWriter, r *http.Request) {
	// TODO: push_token(또는 격자) + 출퇴근 시각으로 단기예보 조회·가공.
	//       출근/퇴근 카드 + 점진적 공개용 시간별 슬라이스 반환 (spec §7.1).
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// ForecastNow 는 GET /forecast/now (선택). 초단기실황 "지금 바깥". spec §3.
func (h *Handler) ForecastNow(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
