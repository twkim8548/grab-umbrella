// Command cron 은 Render Cron Job 으로 매 N분 깨어나 출발 30분 전 푸시를 발송한다. spec §5.
//
// 웹 서비스와 별도 서비스로 돌아 spin-down 영향을 받지 않는다.
//
// 흐름 (spec §5):
//  1. "지금부터 ~lead분 뒤가 출근/퇴근"인 기기만 DB에서 SELECT
//  2. 각 기기 위치·시각으로 초단기예보 조회 (출근=집, 퇴근=회사)
//  3. 한 줄 메시지로 압축 → Expo Push 발송
//  4. 중복 방지: last_morning_push_date / last_evening_push_date 기록
package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/twkim8548/grab-umbrella/server/internal/push"
	"github.com/twkim8548/grab-umbrella/server/internal/store"
	"github.com/twkim8548/grab-umbrella/server/internal/weather"
)

// kst 는 한국 표준시. cron 의 now 는 KST 기준이다(commute "HHmm" 가 KST).
var kst = time.FixedZone("KST", 9*60*60)

// expoTokenPrefix 는 정식 Expo 푸시 토큰 접두사다. 이걸로 시작하지 않으면(예: "dev-")
// 실제 발송할 수 없으므로 cron 에서 skip 한다.
const expoTokenPrefix = "ExponentPushToken["

func main() {
	ctx := context.Background()

	st, err := store.New(ctx, mustEnv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	wc := weather.New(mustEnv("KMA_SERVICE_KEY"), env("KMA_BASE_URL",
		"http://apis.data.go.kr/1360000/VilageFcstInfoService_2.0"))
	pc := push.New(env("EXPO_PUSH_URL", "https://exp.host/--/api/v2/push/send"))

	lead := envInt("PUSH_LEAD_MINUTES", 30)

	now := time.Now().In(kst)
	today := now.Format("20060102")
	log.Printf("cron: tick start (now=%s lead=%dm)", now.Format(time.RFC3339), lead)

	due, err := st.DueDevices(ctx, now, lead)
	if err != nil {
		log.Fatalf("cron: DueDevices: %v", err)
	}
	log.Printf("cron: %d device(s) due", len(due))

	sent, skipped, failed := 0, 0, 0
	for _, d := range due {
		if err := process(ctx, st, wc, pc, d, now, today, &sent, &skipped); err != nil {
			failed++
			log.Printf("cron: device %s slot=%s error: %v", maskToken(d.PushToken), d.Slot, err)
		}
	}

	log.Printf("cron: tick done (due=%d sent=%d skipped=%d failed=%d)",
		len(due), sent, skipped, failed)
}

// process 는 한 기기 한 슬롯을 처리한다. 예보 조회 → 메시지 압축 → (정식 토큰이면) 발송 →
// 발송 성공 시 MarkPushed. 한 기기 실패가 전체를 멈추지 않게 에러는 반환만 한다.
func process(ctx context.Context, st *store.Store, wc *weather.Client, pc *push.Client,
	d store.DueDevice, now time.Time, today string, sent, skipped *int) error {

	fcstDate, fcstTime := weather.SlotDateTime(now, d.FcstTime)

	items, err := fetchForecast(ctx, wc, d.Nx, d.Ny, now, fcstDate, fcstTime)
	if err != nil {
		return err
	}

	slot, ok := weather.SlotForecastAt(items, fcstDate, fcstTime)
	if !ok {
		log.Printf("cron: device %s slot=%s no forecast for %s %s — skip",
			maskToken(d.PushToken), d.Slot, fcstDate, fcstTime)
		*skipped++
		return nil
	}

	title, body, shouldSend := buildMessage(d.Slot, slot)
	if !shouldSend {
		// 비 안 오면 발송 안 함(spec §5). 체감(옷차림) 기반 발송은 §9-7 미구현.
		// TODO(§9-7): 어제 대비 체감 변화가 크면 우산 무관하게 발송.
		*skipped++
		return nil
	}

	// 정식 Expo 토큰만 실제 발송. dev 토큰은 skip(에러 아님)하고 MarkPushed 도 하지 않아
	// 나중에 실제 토큰이 붙으면 받을 수 있게 한다.
	if !hasExpoTokenPrefix(d.PushToken) {
		log.Printf("cron: device %s slot=%s non-expo token — skip send (no mark)",
			maskToken(d.PushToken), d.Slot)
		*skipped++
		return nil
	}

	if err := pc.Send(ctx, push.Message{To: d.PushToken, Title: title, Body: body}); err != nil {
		return err
	}
	if err := st.MarkPushed(ctx, d.PushToken, d.Slot, today); err != nil {
		return err
	}
	*sent++
	log.Printf("cron: device %s slot=%s sent: %s", maskToken(d.PushToken), d.Slot, body)
	return nil
}

// fetchForecast 는 6시간 이내(초단기 범위)면 초단기예보를, 아니면 단기예보를 조회한다.
// 초단기 조회 실패 시 단기예보로 폴백한다(spec §9-2 graceful).
func fetchForecast(ctx context.Context, wc *weather.Client, nx, ny int,
	now time.Time, fcstDate, fcstTime string) ([]weather.FcstItem, error) {

	if weather.WithinUltraRange(now, fcstDate, fcstTime) {
		items, err := wc.UltraSrtForecast(ctx, nx, ny)
		if err == nil {
			return items, nil
		}
		log.Printf("cron: ultra forecast failed (%v) — falling back to vilage", err)
	}
	return wc.VilageForecast(ctx, nx, ny)
}

// buildMessage 는 슬롯 예보를 한 줄 알림으로 압축한다(spec §5). 우산이 필요할 때만
// 발송한다(shouldSend). 비 안 오면 shouldSend=false 로 발송을 거른다.
func buildMessage(slot string, f weather.SlotForecast) (title, body string, shouldSend bool) {
	if !f.NeedUmbrella {
		return "", "", false
	}
	title = "우산챙겨?"
	switch slot {
	case store.SlotMorning:
		body = "오늘 출근길 비 소식, 우산 챙기세요"
	case store.SlotEvening:
		body = "오늘 퇴근길 비 소식, 우산 챙기세요"
	default:
		body = "비 소식, 우산 챙기세요"
	}
	return title, body, true
}

// hasExpoTokenPrefix 는 정식 Expo 토큰(ExponentPushToken[...])인지 본다.
func hasExpoTokenPrefix(token string) bool {
	return len(token) >= len(expoTokenPrefix) && token[:len(expoTokenPrefix)] == expoTokenPrefix
}

// maskToken 은 로그에 토큰 전체를 남기지 않도록 앞부분만 보인다.
func maskToken(token string) string {
	const n = 12
	if len(token) <= n {
		return token
	}
	return token[:n] + "…"
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Printf("cron: invalid %s=%q, using default %d", key, v, def)
		return def
	}
	return n
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required env: %s", key)
	}
	return v
}
