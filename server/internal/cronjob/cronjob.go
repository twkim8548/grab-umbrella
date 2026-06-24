// Package cronjob 은 "출발 lead분 전 우산 푸시" 한 틱의 로직을 담는다(spec §5).
// cmd/cron(독립 실행)과 api 의 /cron/tick(EventBridge 트리거) 양쪽에서 재사용한다.
//
// 흐름: DueDevices(now,lead) → 각 기기 예보 조회 → 윈도우 우산 판정 →
//       비 올 때만(정식 토큰만) Expo 발송 → MarkPushed(중복 방지).
package cronjob

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/twkim8548/grab-umbrella/server/internal/push"
	"github.com/twkim8548/grab-umbrella/server/internal/store"
	"github.com/twkim8548/grab-umbrella/server/internal/weather"
)

// kst 는 한국 표준시. commute "HHmm" 가 KST 기준이라 now 도 KST 로 다룬다.
var kst = time.FixedZone("KST", 9*60*60)

// expoTokenPrefix 는 정식 Expo 푸시 토큰 접두사. dev 토큰("dev-")은 발송 skip.
const expoTokenPrefix = "ExponentPushToken["

// Deps 는 한 틱 실행에 필요한 의존성이다.
type Deps struct {
	Store   *store.Store
	Weather *weather.Client
	Push    *push.Client
}

// Options 는 한 틱의 동작 옵션이다.
type Options struct {
	Now       time.Time // 기준 시각(KST). zero 면 호출부에서 time.Now 로 채운다.
	LeadMin   int       // 출발 몇 분 전(기본 30)
	ForceSend bool      // true 면 비 여부 무관 강제 발송(테스트용)
}

// Result 는 한 틱의 집계 결과다.
type Result struct {
	Due     int
	Sent    int
	Skipped int
	Failed  int
}

// Run 은 한 틱을 실행한다. 한 기기 실패가 전체를 멈추지 않게 개별 에러는 집계만 한다.
// 전체를 멈추는 치명 에러(DueDevices 실패 등)만 error 로 반환한다.
func Run(ctx context.Context, d Deps, o Options) (Result, error) {
	now := o.Now
	if now.IsZero() {
		now = time.Now()
	}
	now = now.In(kst)
	today := now.Format("20060102")
	lead := o.LeadMin
	if lead <= 0 {
		lead = 30
	}
	log.Printf("cron: tick start (now=%s lead=%dm force=%v)", now.Format(time.RFC3339), lead, o.ForceSend)

	due, err := d.Store.DueDevices(ctx, now, lead)
	if err != nil {
		return Result{}, fmt.Errorf("DueDevices: %w", err)
	}
	res := Result{Due: len(due)}
	log.Printf("cron: %d device(s) due", len(due))

	for _, dev := range due {
		if err := process(ctx, d, o, dev, now, today, &res); err != nil {
			res.Failed++
			log.Printf("cron: device %s slot=%s error: %v", maskToken(dev.PushToken), dev.Slot, err)
		}
	}
	log.Printf("cron: tick done (due=%d sent=%d skipped=%d failed=%d)",
		res.Due, res.Sent, res.Skipped, res.Failed)
	return res, nil
}

func process(ctx context.Context, d Deps, o Options, dev store.DueDevice,
	now time.Time, today string, res *Result) error {

	fcstDate, fcstTime := weather.SlotDateTime(now, dev.FcstTime)

	items, err := fetchForecast(ctx, d.Weather, dev.Nx, dev.Ny, now, fcstDate, fcstTime)
	if err != nil {
		return err
	}

	if _, ok := weather.SlotForecastAt(items, fcstDate, fcstTime); !ok {
		log.Printf("cron: device %s slot=%s no forecast for %s %s — skip",
			maskToken(dev.PushToken), dev.Slot, fcstDate, fcstTime)
		res.Skipped++
		return nil
	}

	// 우산 판정은 출퇴근 윈도우 전체 기준(앱 /forecast 와 동일). 출근=morning, 퇴근=evening.
	before, after := weather.MorningWindow()
	if dev.Slot == store.SlotEvening {
		before, after = weather.EveningWindow()
	}
	needUmbrella := weather.WindowNeedUmbrella(items, fcstDate, fcstTime, before, after)

	title, body, shouldSend := buildMessage(dev.Slot, needUmbrella)
	if o.ForceSend && !shouldSend {
		title, body, shouldSend = forceMessage(dev.Slot)
		log.Printf("cron: ⚠ force send for %s", dev.Slot)
	}
	if !shouldSend {
		// 비 안 오면 발송 안 함(spec §5). 체감 기반 발송은 §9-7 미구현.
		res.Skipped++
		return nil
	}

	// 정식 Expo 토큰만 실제 발송. dev 토큰은 skip(MarkPushed 도 안 함 → 나중에 정식 토큰 붙으면 수신).
	if !hasExpoTokenPrefix(dev.PushToken) {
		log.Printf("cron: device %s slot=%s non-expo token — skip send", maskToken(dev.PushToken), dev.Slot)
		res.Skipped++
		return nil
	}

	if err := d.Push.Send(ctx, push.Message{To: dev.PushToken, Title: title, Body: body}); err != nil {
		return err
	}
	if err := d.Store.MarkPushed(ctx, dev.PushToken, dev.Slot, today); err != nil {
		return err
	}
	res.Sent++
	log.Printf("cron: device %s slot=%s sent: %s", maskToken(dev.PushToken), dev.Slot, body)
	return nil
}

// fetchForecast 는 6시간 이내면 초단기예보, 아니면 단기예보. 초단기 실패 시 단기로 폴백(§9-2).
func fetchForecast(ctx context.Context, wc *weather.Client, nx, ny int,
	now time.Time, fcstDate, fcstTime string) ([]weather.FcstItem, error) {

	if weather.WithinUltraRange(now, fcstDate, fcstTime) {
		if items, err := wc.UltraSrtForecast(ctx, nx, ny); err == nil {
			return items, nil
		} else {
			log.Printf("cron: ultra forecast failed (%v) — falling back to vilage", err)
		}
	}
	return wc.VilageForecast(ctx, nx, ny)
}

// buildMessage 는 슬롯 예보를 한 줄 알림으로 압축한다(spec §5). 비 올 때만 발송.
func buildMessage(slot string, needUmbrella bool) (title, body string, shouldSend bool) {
	if !needUmbrella {
		return "", "", false
	}
	return msgFor(slot)
}

// forceMessage 는 ForceSend 테스트 시 비 여부 무관 문구. buildMessage 비 케이스와 동일.
func forceMessage(slot string) (title, body string, shouldSend bool) {
	return msgFor(slot)
}

func msgFor(slot string) (title, body string, shouldSend bool) {
	title = "우산 챙기세요! ☔️"
	switch slot {
	case store.SlotMorning:
		body = "오늘 출근길에 비소식이 있어요"
	case store.SlotEvening:
		body = "오늘 퇴근길에 비소식이 있어요"
	default:
		body = "오늘 비소식이 있어요"
	}
	return title, body, true
}

func hasExpoTokenPrefix(token string) bool {
	return len(token) >= len(expoTokenPrefix) && token[:len(expoTokenPrefix)] == expoTokenPrefix
}

func maskToken(token string) string {
	const n = 12
	if len(token) <= n {
		return token
	}
	return token[:n] + "…"
}
