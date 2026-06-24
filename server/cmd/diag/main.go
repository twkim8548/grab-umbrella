// Command diag 는 개발용 일회성 진단 도구다. 특정 격자/시각에서 단기·초단기 예보가
// 어떤 슬롯을 반환하는지 본다. cron "no forecast" 원인 추적용. 운영 배포 대상 아님.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"github.com/twkim8548/grab-umbrella/server/internal/weather"
)

var kst = time.FixedZone("KST", 9*60*60)

func main() {
	ctx := context.Background()
	wc := weather.New(os.Getenv("KMA_SERVICE_KEY"), env("KMA_BASE_URL",
		"http://apis.data.go.kr/1360000/VilageFcstInfoService_2.0"))

	// 사용자 집 격자(용인 수지) — sync 시 grid.ToGrid 로 계산된 값.
	// 정확한 값을 모르면 인자로 받는다: diag <nx> <ny> [HHmm]
	nx, ny := 62, 122 // 용인 수지 근방 기본값(틀리면 인자로 덮어씀)
	slot := "0700"
	if len(os.Args) >= 3 {
		fmt.Sscanf(os.Args[1], "%d", &nx)
		fmt.Sscanf(os.Args[2], "%d", &ny)
	}
	if len(os.Args) >= 4 {
		slot = os.Args[3]
	}

	fmt.Printf("=== 격자(%d,%d) 단기예보 슬롯 점검 (찾는 슬롯=%s) ===\n", nx, ny, slot)

	items, err := wc.VilageForecast(ctx, nx, ny)
	if err != nil {
		log.Fatalf("VilageForecast: %v", err)
	}
	fmt.Printf("총 %d개 item 수신\n", len(items))

	// fcstDate+fcstTime 별로 어떤 시각 슬롯들이 있는지 집계.
	seen := map[string]bool{}
	for _, it := range items {
		seen[it.FcstDate+" "+it.FcstTime] = true
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Printf("제공된 (날짜 시각) 슬롯 %d개:\n", len(keys))
	for _, k := range keys {
		fmt.Printf("  %s\n", k)
	}

	// 오늘 0700 슬롯이 실제로 잡히는지.
	today := time.Now().In(kst).Format("20060102")
	f, ok := weather.SlotForecastAt(items, today, slot)
	fmt.Printf("\nSlotForecastAt(%s, %s) → ok=%v", today, slot, ok)
	if ok {
		fmt.Printf("  needUmbrella=%v pop=%d%% sky=%q pty=%q\n", f.NeedUmbrella, f.PopPct, f.SkyText, f.PtyText)
	} else {
		fmt.Println("  (해당 슬롯 없음)")
	}
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
