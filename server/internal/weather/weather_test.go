package weather

import "testing"
import "time"

func TestBaseTime(t *testing.T) {
	mk := func(y int, mo time.Month, d, h, mi int) time.Time {
		return time.Date(y, mo, d, h, mi, 0, 0, kst)
	}

	cases := []struct {
		name     string
		now      time.Time
		wantDate string
		wantTime string
	}{
		// 마진 15분(가이드 "발표 후 10분 이후" + 여유). 이하 케이스는 이 기준으로 검증한다.
		// 08:10 → 08시 발표본은 15분 미경과(08:15 필요) → 05시 발표본.
		{"0810 uses 0500", mk(2026, 6, 23, 8, 10), "20260623", "0500"},
		// 08:20 → 08시 발표본 15분 경과(08:15) → 08시 발표본.
		{"0820 uses 0800", mk(2026, 6, 23, 8, 20), "20260623", "0800"},
		// 정확히 08:15 → 마진 경계 포함(>=) → 08시 발표본.
		{"0815 boundary uses 0800", mk(2026, 6, 23, 8, 15), "20260623", "0800"},
		// 08:14 → 아직 미경과 → 05시 발표본.
		{"0814 uses 0500", mk(2026, 6, 23, 8, 14), "20260623", "0500"},
		// 00:30 → 전날 23시 발표본, base_date 어제.
		{"0030 uses prev 2300", mk(2026, 6, 23, 0, 30), "20260622", "2300"},
		// 02:14 → 아직 02시 발표본 미경과(02:15 필요) → 전날 23시.
		{"0214 uses prev 2300", mk(2026, 6, 23, 2, 14), "20260622", "2300"},
		// 02:15 → 02시 발표본 사용 가능.
		{"0215 uses 0200", mk(2026, 6, 23, 2, 15), "20260623", "0200"},
		// 14:20 → 14시 발표본(14:15 경과). 라이브 검증에서 11시로 후퇴하던 케이스 정정.
		{"1420 uses 1400", mk(2026, 6, 23, 14, 20), "20260623", "1400"},
		// 23:50 → 23시 발표본.
		{"2350 uses 2300", mk(2026, 6, 23, 23, 50), "20260623", "2300"},
		// 23:14 → 23시 발표본 미경과(23:15) → 20시 발표본.
		{"2314 uses 2000", mk(2026, 6, 23, 23, 14), "20260623", "2000"},
		// 월 경계: 7월 1일 00:30 → 6월 30일 23시.
		{"month boundary 0030", mk(2026, 7, 1, 0, 30), "20260630", "2300"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotDate, gotTime := BaseTime(tc.now)
			if gotDate != tc.wantDate || gotTime != tc.wantTime {
				t.Errorf("BaseTime(%s) = (%s, %s); want (%s, %s)",
					tc.now.Format(time.RFC3339), gotDate, gotTime, tc.wantDate, tc.wantTime)
			}
		})
	}
}

func TestBaseTimeNonKSTInput(t *testing.T) {
	// UTC 입력도 KST 로 변환해 동작해야 한다.
	// 2026-06-23 00:00 UTC == 2026-06-23 09:00 KST → 08시 발표본(09:00 >= 08:15).
	utc := time.Date(2026, 6, 23, 0, 0, 0, 0, time.UTC)
	gotDate, gotTime := BaseTime(utc)
	if gotDate != "20260623" || gotTime != "0800" {
		t.Errorf("BaseTime(UTC 0000) = (%s, %s); want (20260623, 0800)", gotDate, gotTime)
	}
}

func TestUltraSrtBaseTime(t *testing.T) {
	mk := func(y int, mo time.Month, d, h, mi int) time.Time {
		return time.Date(y, mo, d, h, mi, 0, 0, kst)
	}

	cases := []struct {
		name     string
		now      time.Time
		wantDate string
		wantTime string
	}{
		// 분 >= 45 → 이번 시각 30분 발표본.
		{"1650 uses 1630", mk(2026, 6, 23, 16, 50), "20260623", "1630"},
		// 분 < 45 → 한 시간 전 30분 발표본.
		{"1620 uses 1530", mk(2026, 6, 23, 16, 20), "20260623", "1530"},
		// 정확히 분=45 경계(>=) → 이번 시각.
		{"1645 boundary uses 1630", mk(2026, 6, 23, 16, 45), "20260623", "1630"},
		// 분=44 → 직전 시각.
		{"1644 uses 1530", mk(2026, 6, 23, 16, 44), "20260623", "1530"},
		// 분=00 → 직전 시각.
		{"1600 uses 1530", mk(2026, 6, 23, 16, 0), "20260623", "1530"},
		// 자정 경계: 00:20 → 전날 2330.
		{"0020 uses prev 2330", mk(2026, 6, 23, 0, 20), "20260622", "2330"},
		// 00:50 → 당일 0030.
		{"0050 uses 0030", mk(2026, 6, 23, 0, 50), "20260623", "0030"},
		// 월 경계: 7월 1일 00:10 → 6월 30일 2330.
		{"month boundary 0010", mk(2026, 7, 1, 0, 10), "20260630", "2330"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotDate, gotTime := UltraSrtBaseTime(tc.now)
			if gotDate != tc.wantDate || gotTime != tc.wantTime {
				t.Errorf("UltraSrtBaseTime(%s) = (%s, %s); want (%s, %s)",
					tc.now.Format(time.RFC3339), gotDate, gotTime, tc.wantDate, tc.wantTime)
			}
		})
	}
}

func TestUltraSrtBaseTimeNonKSTInput(t *testing.T) {
	// 2026-06-23 00:00 UTC == 2026-06-23 09:00 KST → 분 0 < 45 → 0830 발표본.
	utc := time.Date(2026, 6, 23, 0, 0, 0, 0, time.UTC)
	gotDate, gotTime := UltraSrtBaseTime(utc)
	if gotDate != "20260623" || gotTime != "0830" {
		t.Errorf("UltraSrtBaseTime(UTC 0000) = (%s, %s); want (20260623, 0830)", gotDate, gotTime)
	}
}

// TestSlotForecastAtT1HFallback 은 초단기예보 items(기온 T1H, TMP 없음)에서 기온이 T1H 로
// 폴백되는지 검증한다. SKY/PTY/POP 는 단기와 동일 코드명이라 그대로 처리된다.
func TestSlotForecastAtT1HFallback(t *testing.T) {
	const ultraResponse = `{
	  "response": {
	    "header": {"resultCode": "00", "resultMsg": "NORMAL_SERVICE"},
	    "body": {"items": {"item": [
	      {"category":"T1H","fcstDate":"20260623","fcstTime":"1500","fcstValue":"29","nx":60,"ny":127},
	      {"category":"SKY","fcstDate":"20260623","fcstTime":"1500","fcstValue":"4","nx":60,"ny":127},
	      {"category":"PTY","fcstDate":"20260623","fcstTime":"1500","fcstValue":"1","nx":60,"ny":127},
	      {"category":"POP","fcstDate":"20260623","fcstTime":"1500","fcstValue":"80","nx":60,"ny":127},
	      {"category":"RN1","fcstDate":"20260623","fcstTime":"1500","fcstValue":"1.0","nx":60,"ny":127}
	    ]}}
	  }
	}`
	items, err := parseVilageFcst([]byte(ultraResponse))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	slot, ok := SlotForecastAt(items, "20260623", "1500")
	if !ok {
		t.Fatal("expected slot found")
	}
	// 기온은 TMP 없이 T1H(29)로 폴백.
	if slot.TempC != 29 {
		t.Errorf("TempC = %d; want 29 (T1H fallback)", slot.TempC)
	}
	if slot.SkyText != "흐림" || slot.PtyText != "비" || slot.PopPct != 80 || !slot.NeedUmbrella {
		t.Errorf("slot = %+v", slot)
	}
}

// TestTempCPrefersTMP 는 TMP·T1H 가 둘 다 있으면 TMP 를 우선하는지 검증한다.
func TestTempCPrefersTMP(t *testing.T) {
	if got := tempC(map[string]string{"TMP": "25", "T1H": "30"}); got != 25 {
		t.Errorf("tempC(TMP=25,T1H=30) = %d; want 25", got)
	}
	if got := tempC(map[string]string{"T1H": "30"}); got != 30 {
		t.Errorf("tempC(T1H=30) = %d; want 30", got)
	}
	if got := tempC(map[string]string{}); got != 0 {
		t.Errorf("tempC(empty) = %d; want 0", got)
	}
}

func TestSkyText(t *testing.T) {
	cases := map[int]string{1: "맑음", 3: "구름많음", 4: "흐림", 99: ""}
	for code, want := range cases {
		if got := skyText(code); got != want {
			t.Errorf("skyText(%d) = %q; want %q", code, got, want)
		}
	}
}

func TestPtyText(t *testing.T) {
	cases := map[int]string{0: "없음", 1: "비", 2: "비/눈", 3: "눈", 4: "소나기", 99: ""}
	for code, want := range cases {
		if got := ptyText(code); got != want {
			t.Errorf("ptyText(%d) = %q; want %q", code, got, want)
		}
	}
}

func TestNeedUmbrella(t *testing.T) {
	cases := []struct {
		pty, pop int
		want     bool
	}{
		{0, 0, false},
		{0, 59, false},
		{0, 60, true}, // POP 임계값 도달.
		{0, 100, true},
		{1, 0, true},  // 비 → 확률 무관.
		{3, 10, true}, // 눈.
		{4, 5, true},  // 소나기.
	}
	for _, tc := range cases {
		if got := NeedUmbrella(tc.pty, tc.pop); got != tc.want {
			t.Errorf("NeedUmbrella(%d, %d) = %v; want %v", tc.pty, tc.pop, got, tc.want)
		}
	}
}

// sampleResponse 는 getVilageFcst 실제 응답 형태의 축약 샘플이다(nx=60, ny=127).
const sampleResponse = `{
  "response": {
    "header": {"resultCode": "00", "resultMsg": "NORMAL_SERVICE"},
    "body": {
      "items": {
        "item": [
          {"baseDate":"20260622","baseTime":"2300","category":"TMP","fcstDate":"20260623","fcstTime":"0900","fcstValue":"23","nx":60,"ny":127},
          {"baseDate":"20260622","baseTime":"2300","category":"SKY","fcstDate":"20260623","fcstTime":"0900","fcstValue":"3","nx":60,"ny":127},
          {"baseDate":"20260622","baseTime":"2300","category":"PTY","fcstDate":"20260623","fcstTime":"0900","fcstValue":"1","nx":60,"ny":127},
          {"baseDate":"20260622","baseTime":"2300","category":"POP","fcstDate":"20260623","fcstTime":"0900","fcstValue":"70","nx":60,"ny":127},
          {"baseDate":"20260622","baseTime":"2300","category":"TMP","fcstDate":"20260623","fcstTime":"1800","fcstValue":"27","nx":60,"ny":127},
          {"baseDate":"20260622","baseTime":"2300","category":"SKY","fcstDate":"20260623","fcstTime":"1800","fcstValue":"1","nx":60,"ny":127},
          {"baseDate":"20260622","baseTime":"2300","category":"PTY","fcstDate":"20260623","fcstTime":"1800","fcstValue":"0","nx":60,"ny":127},
          {"baseDate":"20260622","baseTime":"2300","category":"POP","fcstDate":"20260623","fcstTime":"1800","fcstValue":"20","nx":60,"ny":127}
        ]
      }
    }
  }
}`

func TestParseVilageFcst(t *testing.T) {
	items, err := parseVilageFcst([]byte(sampleResponse))
	if err != nil {
		t.Fatalf("parseVilageFcst error: %v", err)
	}
	if len(items) != 8 {
		t.Fatalf("got %d items; want 8", len(items))
	}
	if items[0].Category != "TMP" || items[0].FcstValue != "23" || items[0].Nx != 60 {
		t.Errorf("unexpected first item: %+v", items[0])
	}
}

func TestParseVilageFcstAPIError(t *testing.T) {
	body := `{"response":{"header":{"resultCode":"03","resultMsg":"NODATA_ERROR"},"body":{"items":{"item":[]}}}}`
	if _, err := parseVilageFcst([]byte(body)); err == nil {
		t.Fatal("expected error for non-00 resultCode, got nil")
	}
}

func TestSlotForecastAt(t *testing.T) {
	items, err := parseVilageFcst([]byte(sampleResponse))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// 출근(0900): 구름많음 + 비 + 70% → 우산 필요.
	morning, ok := SlotForecastAt(items, "20260623", "0900")
	if !ok {
		t.Fatal("expected morning slot found")
	}
	if morning.SkyText != "구름많음" || morning.PtyText != "비" || morning.TempC != 23 ||
		morning.PopPct != 70 || !morning.NeedUmbrella {
		t.Errorf("morning = %+v", morning)
	}

	// 퇴근(1800): 맑음 + 없음 + 20% → 우산 불필요.
	evening, ok := SlotForecastAt(items, "20260623", "1800")
	if !ok {
		t.Fatal("expected evening slot found")
	}
	if evening.SkyText != "맑음" || evening.PtyText != "없음" || evening.TempC != 27 ||
		evening.PopPct != 20 || evening.NeedUmbrella {
		t.Errorf("evening = %+v", evening)
	}

	// 없는 시각.
	if _, ok := SlotForecastAt(items, "20260623", "0300"); ok {
		t.Error("expected not found for 0300")
	}
}
