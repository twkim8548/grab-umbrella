package weather

import (
	"testing"
	"time"
)

func TestNormalizeToHour(t *testing.T) {
	cases := map[string]string{
		"0830": "0800",
		"0900": "0900",
		"1830": "1800",
		"0000": "0000",
		"2359": "2300",
		"":     "",     // 형식 아님 → 그대로
		"abc":  "abc",  // 형식 아님 → 그대로
		"09":   "09",   // 길이 아님 → 그대로
		"09a0": "09a0", // 비숫자 → 그대로
	}
	for in, want := range cases {
		if got := NormalizeToHour(in); got != want {
			t.Errorf("NormalizeToHour(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestSlotDateTime(t *testing.T) {
	at := func(h, m int) time.Time {
		return time.Date(2026, 6, 23, h, m, 0, 0, kst)
	}
	cases := []struct {
		name     string
		now      time.Time
		commute  string
		wantDate string
		wantTime string
	}{
		// 출근 0900, 현재 0700 → 아직 안 지남 → 오늘.
		{"before slot -> today", at(7, 0), "0900", "20260623", "0900"},
		// 출근 0900, 현재 1000 → 이미 지남 → 내일.
		{"after slot -> tomorrow", at(10, 0), "0900", "20260624", "0900"},
		// 정확히 슬롯 시각 → "지났다(After)" 아님 → 오늘.
		{"exact slot -> today", at(9, 0), "0900", "20260623", "0900"},
		// 30분 commute 정시 내림 + 미경과 → 오늘 0800.
		{"0830 normalized today", at(7, 0), "0830", "20260623", "0800"},
		// 30분 commute, 현재 0840(>08:30) → 지남 → 내일 0800.
		{"0830 passed -> tomorrow", at(8, 40), "0830", "20260624", "0800"},
		// 퇴근 1800, 현재 1900 → 내일.
		{"evening passed -> tomorrow", at(19, 0), "1800", "20260624", "1800"},
		// 자정 직전 출근(자정 넘김 케이스): commute 2330, 현재 2340 → 내일.
		{"late passed -> next day", at(23, 40), "2330", "20260624", "2300"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gd, gt := SlotDateTime(tc.now, tc.commute)
			if gd != tc.wantDate || gt != tc.wantTime {
				t.Errorf("SlotDateTime(%s, %q) = (%s,%s); want (%s,%s)",
					tc.now.Format("1504"), tc.commute, gd, gt, tc.wantDate, tc.wantTime)
			}
		})
	}
}

func TestSlotDateTimeNonKST(t *testing.T) {
	// UTC 입력도 KST 로 변환. 2026-06-23 00:00 UTC == 09:00 KST.
	// commute 0900, 현재 09:00 KST → 미경과(After 아님) → 오늘.
	utc := time.Date(2026, 6, 23, 0, 0, 0, 0, time.UTC)
	gd, gt := SlotDateTime(utc, "0900")
	if gd != "20260623" || gt != "0900" {
		t.Errorf("SlotDateTime(UTC) = (%s,%s); want (20260623,0900)", gd, gt)
	}
}

func TestWithinUltraRange(t *testing.T) {
	now := time.Date(2026, 6, 23, 15, 0, 0, 0, kst)
	cases := []struct {
		name     string
		fcstDate string
		fcstTime string
		want     bool
	}{
		// 같은 시각(0시간 차) → 범위 내.
		{"now exactly", "20260623", "1500", true},
		// +1시간 → 범위 내.
		{"plus 1h", "20260623", "1600", true},
		// 정확히 +6시간 경계(<=) → 범위 내.
		{"plus 6h boundary", "20260623", "2100", true},
		// +7시간 → 범위 밖.
		{"plus 7h", "20260623", "2200", false},
		// 과거(-1시간) → 범위 밖.
		{"past", "20260623", "1400", false},
		// 자정 넘김 +6시간 이내 검증: now 21:00, slot 익일 02:00(=+5h) → 범위 내.
		{"midnight cross in range", "20260624", "0200", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := WithinUltraRange(now, tc.fcstDate, tc.fcstTime); got != tc.want {
				t.Errorf("WithinUltraRange(15:00, %s %s) = %v; want %v",
					tc.fcstDate, tc.fcstTime, got, tc.want)
			}
		})
	}
}

func TestWithinUltraRangeMidnightCross(t *testing.T) {
	// now 21:00, slot 익일 02:00 = +5시간 → 범위 내(자정 경계 처리).
	now := time.Date(2026, 6, 23, 21, 0, 0, 0, kst)
	if !WithinUltraRange(now, "20260624", "0200") {
		t.Error("expected +5h across midnight to be within ultra range")
	}
	// now 21:00, slot 익일 0400 = +7시간 → 범위 밖.
	if WithinUltraRange(now, "20260624", "0400") {
		t.Error("expected +7h across midnight to be outside ultra range")
	}
}

func TestWithinUltraRangeNonKST(t *testing.T) {
	// 2026-06-23 06:00 UTC == 15:00 KST. slot 1600 KST = +1h → 범위 내.
	utc := time.Date(2026, 6, 23, 6, 0, 0, 0, time.UTC)
	if !WithinUltraRange(utc, "20260623", "1600") {
		t.Error("expected UTC input converted to KST to be within range")
	}
}

// hourlyItems 는 0600~2200 정시별 TMP/POP/PTY 를 채운 두 날짜 데이터다(슬라이스 윈도우 검증용).
func hourlyItems() []FcstItem {
	var items []FcstItem
	add := func(date, tm, cat, val string) {
		items = append(items, FcstItem{FcstDate: date, FcstTime: tm, Category: cat, FcstValue: val, Nx: 60, Ny: 127})
	}
	hours := []string{"0500", "0600", "0700", "0800", "0900", "1000", "1100",
		"1600", "1700", "1800", "1900", "2000", "2200", "2300"}
	for _, h := range hours {
		add("20260623", h, "TMP", "20")
		add("20260623", h, "POP", "10")
		add("20260623", h, "PTY", "0")
	}
	// 다음날 0000, 0100 (퇴근 후 자정 넘김 검증용).
	for _, h := range []string{"0000", "0100"} {
		add("20260624", h, "TMP", "18")
		add("20260624", h, "POP", "5")
		add("20260624", h, "PTY", "0")
	}
	return items
}

func TestHourlySliceMorningAsymmetric(t *testing.T) {
	// 출근 0800, before=2 after=1 → 0600,0700,0800,0900.
	pts := HourlySlice(hourlyItems(), "20260623", "0800", morningHoursBefore, morningHoursAfter)
	want := []string{"0600", "0700", "0800", "0900"}
	assertTimes(t, pts, want)
}

func TestHourlySliceEveningAsymmetric(t *testing.T) {
	// 퇴근 1800, before=1 after=2 → 1700,1800,1900,2000.
	pts := HourlySlice(hourlyItems(), "20260623", "1800", eveningHoursBefore, eveningHoursAfter)
	want := []string{"1700", "1800", "1900", "2000"}
	assertTimes(t, pts, want)
}

func TestHourlySliceMidnightBoundary(t *testing.T) {
	// 퇴근 2300, before=1 after=2 → 2200, 2300, (다음날)0000, 0100.
	// 자정 넘김 시각이 올바른 날짜에서 조회되고 시간 오름차순(2200..0100) 유지되는지 검증.
	pts := HourlySlice(hourlyItems(), "20260623", "2300", 1, 2)
	want := []string{"2200", "2300", "0000", "0100"}
	assertTimes(t, pts, want)
}

func TestHourlySliceSkipsMissing(t *testing.T) {
	// 0800 기준 before=3 → 0500,0600,0700,0800,0900 중 0400 없음은 자연스레 제외.
	// before=4 면 0400 슬롯이 없으므로(데이터 없음) 건너뛴다 → 0500부터 시작.
	pts := HourlySlice(hourlyItems(), "20260623", "0800", 4, 0)
	want := []string{"0500", "0600", "0700", "0800"}
	assertTimes(t, pts, want)
}

func assertTimes(t *testing.T, pts []HourlyPoint, want []string) {
	t.Helper()
	if len(pts) != len(want) {
		t.Fatalf("got %d points %v; want %d %v", len(pts), times(pts), len(want), want)
	}
	for i := range want {
		if pts[i].Time != want[i] {
			t.Errorf("point[%d].Time = %q; want %q (all: %v)", i, pts[i].Time, want[i], times(pts))
		}
	}
}

func times(pts []HourlyPoint) []string {
	out := make([]string, len(pts))
	for i, p := range pts {
		out[i] = p.Time
	}
	return out
}

func TestBuildSlotCard(t *testing.T) {
	items, err := parseVilageFcst([]byte(sampleResponse))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	card, ok := BuildSlotCard(items, "20260623", "0900", 2, 1)
	if !ok {
		t.Fatal("expected card ok")
	}
	if card.SkyText != "구름많음" || card.PtyText != "비" || card.TempC != 23 ||
		card.PopPct != 70 || !card.NeedUmbrella {
		t.Errorf("card = %+v", card)
	}
	if card.Hourly == nil {
		t.Error("hourly must be non-nil slice")
	}

	// 없는 시각 → ok=false.
	if _, ok := BuildSlotCard(items, "20260623", "0300", 2, 1); ok {
		t.Error("expected not ok for missing slot")
	}
}

func TestPrecipText(t *testing.T) {
	cases := map[string]string{
		"":            "강수없음",
		"-":           "강수없음",
		"null":        "강수없음",
		"0":           "강수없음",
		"강수없음":        "강수없음",
		"1.0mm":       "1.0mm",
		"30.0~50.0mm": "30.0~50.0mm",
	}
	for in, want := range cases {
		if got := PrecipText(in); got != want {
			t.Errorf("PrecipText(%q) = %q; want %q", in, got, want)
		}
	}
}
