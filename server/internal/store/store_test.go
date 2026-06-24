package store

import (
	"testing"
	"time"
)

// TestDueSlot 은 순수 윈도우 매칭 로직을 DB 없이 검증한다.
// tick=10분, lead=30분 가정. now+lead 가 commute 시각의 [target, target+10m) 에 들면 due.
func TestDueSlot(t *testing.T) {
	mk := func(h, m int) time.Time {
		return time.Date(2026, 6, 24, h, m, 0, 0, kst)
	}
	const lead = 30
	const tick = 10 * time.Minute

	cases := []struct {
		name       string
		now        time.Time
		start, end string
		want       string
	}{
		// now=08:30, +30m=09:00. commute_start 0900 → window [0900,0910) 포함 → morning.
		{"exact morning", mk(8, 30), "0900", "1800", SlotMorning},
		// now=08:35, +30m=09:05. 0900 in [0905..) ? 0900 < 0905 → 미포함.
		{"just past morning window", mk(8, 35), "0900", "1800", ""},
		// now=08:25, +30m=08:55. 0900 in [0855,0905) → morning.
		{"within window before", mk(8, 25), "0900", "1800", SlotMorning},
		// now=17:30, +30m=18:00. commute_end 1800 → evening.
		{"exact evening", mk(17, 30), "0900", "1800", SlotEvening},
		// now=10:00, +30m=10:30. 아무 commute 도 아님.
		{"none", mk(10, 0), "0900", "1800", ""},
		// 30분 출근(0830): now=08:00 +30m=08:30 → morning.
		{"half-hour commute", mk(8, 0), "0830", "1830", SlotMorning},
		// 잘못된 형식은 무시.
		{"bad format", mk(8, 30), "9:00", "1800", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := dueSlot(tc.now, lead, tick, tc.start, tc.end)
			if got != tc.want {
				t.Errorf("dueSlot(%s, %s, %s) = %q; want %q",
					tc.now.Format("1504"), tc.start, tc.end, got, tc.want)
			}
		})
	}
}

// TestDueSlotEveryTickOnce 은 cron 이 매 tick 마다 정확히 한 번만 morning 을 잡는지 본다.
// 08:00~09:00 을 10분 간격으로 돌면 commute 0900·lead 30 에 대해 08:30 한 tick 만 morning.
func TestDueSlotEveryTickOnce(t *testing.T) {
	const lead = 30
	const tick = 10 * time.Minute
	hits := 0
	for m := 0; m < 60; m += 10 {
		now := time.Date(2026, 6, 24, 8, m, 0, 0, kst)
		if dueSlot(now, lead, tick, "0900", "1800") == SlotMorning {
			hits++
		}
	}
	if hits != 1 {
		t.Errorf("morning hit %d times across one hour of ticks; want exactly 1", hits)
	}
}

func TestHHmmToMinutes(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"0900", 540, true},
		{"1830", 1110, true},
		{"0000", 0, true},
		{"2359", 1439, true},
		{"2400", 0, false},
		{"0960", 0, false},
		{"abc", 0, false},
		{"900", 0, false},
	}
	for _, tc := range cases {
		got, ok := hhmmToMinutes(tc.in)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("hhmmToMinutes(%q) = (%d,%v); want (%d,%v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}
