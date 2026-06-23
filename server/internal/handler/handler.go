// Package handler 는 HTTP 핸들러를 모은다. spec §3 API 3개.
package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/twkim8548/grab-umbrella/server/internal/geocode"
	"github.com/twkim8548/grab-umbrella/server/internal/grid"
	"github.com/twkim8548/grab-umbrella/server/internal/store"
	"github.com/twkim8548/grab-umbrella/server/internal/weather"
)

type Handler struct {
	Store   *store.Store
	Weather *weather.Client
	Geocode *geocode.Client
}

// syncRequest — POST /sync 입력. 앱이 도로명 주소를 보내면 서버가 위경도→격자로 변환.
// 변환 로직은 서버 한 곳에 둔다(spec §2).
type syncRequest struct {
	PushToken    string `json:"push_token"`
	HomeAddress  string `json:"home_address"`
	WorkAddress  string `json:"work_address"`
	CommuteStart string `json:"commute_start"` // "0900"
	CommuteEnd   string `json:"commute_end"`   // "1800"
}

// Sync 는 POST /sync. 주소→위경도(카카오)→격자 변환 후 devices upsert. spec §2·§3.
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
	if req.HomeAddress == "" || req.WorkAddress == "" {
		http.Error(w, "home_address and work_address required", http.StatusBadRequest)
		return
	}

	homeLat, homeLng, err := h.Geocode.Geocode(r.Context(), req.HomeAddress)
	if err != nil {
		log.Printf("sync: geocode home %q: %v", req.HomeAddress, err)
		http.Error(w, "집 주소를 찾을 수 없습니다", http.StatusUnprocessableEntity)
		return
	}
	workLat, workLng, err := h.Geocode.Geocode(r.Context(), req.WorkAddress)
	if err != nil {
		log.Printf("sync: geocode work %q: %v", req.WorkAddress, err)
		http.Error(w, "회사 주소를 찾을 수 없습니다", http.StatusUnprocessableEntity)
		return
	}

	hnx, hny := grid.ToGrid(homeLat, homeLng)
	wnx, wny := grid.ToGrid(workLat, workLng)

	d := store.Device{
		PushToken:    req.PushToken,
		HomeNx:       hnx,
		HomeNy:       hny,
		WorkNx:       wnx,
		WorkNy:       wny,
		HomeAddress:  req.HomeAddress,
		WorkAddress:  req.WorkAddress,
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
