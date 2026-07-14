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
	"github.com/twkim8548/grab-umbrella/server/internal/push"
	"github.com/twkim8548/grab-umbrella/server/internal/store"
	"github.com/twkim8548/grab-umbrella/server/internal/weather"
)

type Handler struct {
	Store   *store.Store
	Weather *weather.Client
	Geocode *geocode.Client
	Push    *push.Client // /cron/tick 푸시 발송용
	// CronSecret 은 /cron/tick 호출을 보호하는 공유 시크릿. 비어 있으면 /cron/tick 비활성.
	CronSecret string
}

// syncRequest — POST /sync 입력. 앱이 도로명 주소를 보내면 서버가 위경도→격자로 변환.
// 변환 로직은 서버 한 곳에 둔다(spec §2).
type syncRequest struct {
	PushToken    string `json:"push_token"`
	HomeAddress  string `json:"home_address"`
	WorkAddress  string `json:"work_address"`
	CommuteStart string `json:"commute_start"` // "0900"
	CommuteEnd   string `json:"commute_end"`   // "1800"
	CommuteDays  string `json:"commute_days"`  // "0111110" 일~토, 1=on. 빈값/형식오류면 평일 기본.
	// 포인터로 필드 생략(구버전 앱)과 명시적인 false 를 구분한다.
	NotificationsEnabled *bool `json:"notifications_enabled"`
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
		PushToken:            req.PushToken,
		HomeNx:               hnx,
		HomeNy:               hny,
		WorkNx:               wnx,
		WorkNy:               wny,
		HomeAddress:          req.HomeAddress,
		WorkAddress:          req.WorkAddress,
		CommuteStart:         req.CommuteStart,
		CommuteEnd:           req.CommuteEnd,
		CommuteDays:          normalizeDays(req.CommuteDays),
		NotificationsEnabled: notificationsEnabled(req.NotificationsEnabled),
	}
	if err := h.Store.Upsert(r.Context(), d); err != nil {
		http.Error(w, "upsert failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// dayForecast 는 하루치(출근/퇴근) 카드다. 데이터 없거나 이미 지난 시점은 null.
type dayForecast struct {
	Morning *weather.SlotCard `json:"morning"`
	Evening *weather.SlotCard `json:"evening"`
}

// forecastResponse 는 GET /forecast 응답이다. 오늘·내일 출퇴근 4시점을 모두 내린다.
// 데이터가 없거나 이미 지난 슬롯은 null 로 내려 앱이 현재 시각에 맞춰 판정하게 한다.
type forecastResponse struct {
	Today    dayForecast `json:"today"`
	Tomorrow dayForecast `json:"tomorrow"`
}

// Forecast 는 GET /forecast. 오늘·내일의 출근/퇴근 4시점 카드 + 시간별 흐름. spec §3·§7.1.
//
// 앱이 "내일"을 하루 단위(출근+퇴근)로 판정할 수 있도록, 출퇴근 사이 시간대에도
// 4시점(오늘 출근/퇴근, 내일 출근/퇴근)을 모두 반환한다. 이미 지난 시점은 null.
func (h *Handler) Forecast(w http.ResponseWriter, r *http.Request) {
	// 개발용 시나리오 프리뷰: ?mock=... 이 있으면 DB·기상청 없이 가짜 4시점을 내린다.
	// 운영 영향 없음(파라미터 없으면 통과). 어휘는 internal/handler/mock.go 참고.
	if mock := r.URL.Query().Get("mock"); mock != "" {
		log.Printf("forecast: ⚠ mock preview (%q)", mock)
		writeMockForecast(w, mock)
		return
	}

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

	now := time.Now().In(time.FixedZone("KST", 9*60*60))
	today := now.Format("20060102")
	tomorrow := now.AddDate(0, 0, 1).Format("20060102")

	mBefore, mAfter := weather.MorningWindow()
	eBefore, eAfter := weather.EveningWindow()

	// 집 격자 = 출근 카드, 회사 격자 = 퇴근 카드. 같은 격자의 단기예보는 weather 내부
	// 캐시(같은 발표본)로 공유되므로, 4시점이 격자별 1회 호출로 커버된다(단기는 글피까지 제공).
	var resp forecastResponse
	if card, ok := h.buildCardForDate(r.Context(), now, "today.morning", today, dev.CommuteStart, dev.HomeNx, dev.HomeNy, mBefore, mAfter); ok {
		resp.Today.Morning = &card
	}
	if card, ok := h.buildCardForDate(r.Context(), now, "today.evening", today, dev.CommuteEnd, dev.WorkNx, dev.WorkNy, eBefore, eAfter); ok {
		resp.Today.Evening = &card
	}
	if card, ok := h.buildCardForDate(r.Context(), now, "tomorrow.morning", tomorrow, dev.CommuteStart, dev.HomeNx, dev.HomeNy, mBefore, mAfter); ok {
		resp.Tomorrow.Morning = &card
	}
	if card, ok := h.buildCardForDate(r.Context(), now, "tomorrow.evening", tomorrow, dev.CommuteEnd, dev.WorkNx, dev.WorkNy, eBefore, eAfter); ok {
		resp.Tomorrow.Evening = &card
	}

	writeJSON(w, resp)
}

// buildCardForDate 는 명시한 날짜(fcstDate "YYYYMMDD")의 한 슬롯 카드를 만든다(spec §4.1).
//
// 슬롯 시각은 fcstDate + NormalizeToHour(commute) 정시다. 선택 규칙:
//  1. 그 시각이 now(KST)보다 과거면 → nil,false (이미 지난 시점은 표시하지 않는다).
//  2. 지금부터 6시간 이내면 초단기예보(getUltraSrtFcst)를 우선 시도.
//  3. 초단기 호출 실패, 또는 해당 시각 슬롯이 비면 → 단기예보(getVilageFcst)로 폴백.
//  4. 6시간 밖이면 처음부터 단기예보.
//
// 단기예보까지 실패하면 ok=false 를 반환해 호출부가 해당 카드를 null 로 graceful 하게
// 내린다(spec §4.6·§9-2). 카드와 hourly 는 같은 소스 기준으로 슬라이스된다.
func (h *Handler) buildCardForDate(ctx context.Context, now time.Time, slot, fcstDate, commute string, nx, ny int, before, after int) (weather.SlotCard, bool) {
	fcstTime := weather.NormalizeToHour(commute)

	if weather.SlotIsPast(now, fcstDate, fcstTime) {
		log.Printf("forecast: %s skipped (past %s %s)", slot, fcstDate, fcstTime)
		return weather.SlotCard{}, false
	}

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

// normalizeDays 는 commute_days 입력을 검증한다. 7자리 0/1 이면 그대로, 아니면(빈값/구버전
// 앱/손상) 평일(월~금)="0111110" 으로 폴백한다. 서버에서 한 번 더 보정해 cron 을 단순하게.
func normalizeDays(s string) string {
	if len(s) != 7 {
		return "0111110"
	}
	for i := 0; i < 7; i++ {
		if s[i] != '0' && s[i] != '1' {
			return "0111110"
		}
	}
	return s
}

// notificationsEnabled 는 notifications_enabled 를 보내지 않는 구버전 앱을 계속
// 알림 활성 상태로 취급한다. 새 앱이 false 를 명시하면 서버 발송을 비활성화한다.
func notificationsEnabled(enabled *bool) bool {
	return enabled == nil || *enabled
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
