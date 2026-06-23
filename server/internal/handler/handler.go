// Package handler 는 HTTP 핸들러를 모은다. spec §3 API 3개.
package handler

import (
	"context"
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

	mBefore, mAfter := weather.MorningWindow()
	eBefore, eAfter := weather.EveningWindow()

	// 집 격자 = 출근 카드, 회사 격자 = 퇴근 카드. 같은 격자면 weather 내부 캐시로 호출 공유.
	var resp forecastResponse
	if card, ok := h.buildSlotCard(r.Context(), now, "morning", dev.HomeNx, dev.HomeNy, mDate, mTime, mBefore, mAfter); ok {
		resp.Morning = &card
	}
	if card, ok := h.buildSlotCard(r.Context(), now, "evening", dev.WorkNx, dev.WorkNy, eDate, eTime, eBefore, eAfter); ok {
		resp.Evening = &card
	}

	writeJSON(w, resp)
}

// buildSlotCard 는 한 슬롯의 예보 소스를 시점에 따라 자동 선택해 카드를 만든다(spec §4.1).
//
// 선택 규칙:
//  1. 슬롯 시각이 지금부터 6시간 이내면 초단기예보(getUltraSrtFcst)를 우선 시도.
//  2. 초단기 호출 실패, 또는 해당 시각 슬롯이 비면 → 단기예보(getVilageFcst)로 폴백.
//  3. 6시간 밖이면 처음부터 단기예보.
//
// 단기예보까지 실패하면 ok=false 를 반환해 호출부가 해당 카드를 null 로 graceful 하게
// 내린다(spec §4.6·§9-2). 카드와 hourly 는 같은 소스 기준으로 슬라이스된다.
func (h *Handler) buildSlotCard(ctx context.Context, now time.Time, slot string, nx, ny int, fcstDate, fcstTime string, before, after int) (weather.SlotCard, bool) {
	if weather.WithinUltraRange(now, fcstDate, fcstTime) {
		items, err := h.Weather.UltraSrtForecast(ctx, nx, ny)
		if err != nil {
			log.Printf("forecast: %s ultra(%d,%d) failed, falling back to vilage: %v", slot, nx, ny, err)
		} else if card, ok := weather.BuildSlotCard(items, fcstDate, fcstTime, before, after); ok {
			log.Printf("forecast: %s using ultra-short forecast (%d,%d @ %s)", slot, nx, ny, fcstTime)
			return card, true
		} else {
			log.Printf("forecast: %s ultra(%d,%d) no slot @ %s, falling back to vilage", slot, nx, ny, fcstTime)
		}
	}

	items, err := h.Weather.VilageForecast(ctx, nx, ny)
	if err != nil {
		log.Printf("forecast: %s vilage(%d,%d) failed: %v", slot, nx, ny, err)
		return weather.SlotCard{}, false
	}
	card, ok := weather.BuildSlotCard(items, fcstDate, fcstTime, before, after)
	if ok {
		log.Printf("forecast: %s using village (short-term) forecast (%d,%d @ %s)", slot, nx, ny, fcstTime)
	}
	return card, ok
}

// ForecastNow 는 GET /forecast/now (선택). 초단기실황 "지금 바깥". spec §3.
func (h *Handler) ForecastNow(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
