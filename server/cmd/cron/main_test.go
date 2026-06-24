package main

import (
	"testing"

	"github.com/twkim8548/grab-umbrella/server/internal/store"
	"github.com/twkim8548/grab-umbrella/server/internal/weather"
)

// TestBuildMessage 는 메시지 압축 규칙을 검증한다(spec §5): 우산 필요할 때만 발송.
func TestBuildMessage(t *testing.T) {
	rain := weather.SlotForecast{PtyText: "비", PopPct: 80, NeedUmbrella: true}
	dry := weather.SlotForecast{PtyText: "없음", PopPct: 10, NeedUmbrella: false}

	t.Run("morning rain sends", func(t *testing.T) {
		title, body, send := buildMessage(store.SlotMorning, rain)
		if !send {
			t.Fatal("expected shouldSend=true for rain")
		}
		if title != "우산챙겨?" {
			t.Errorf("title = %q", title)
		}
		if body != "오늘 출근길 비 소식, 우산 챙기세요" {
			t.Errorf("body = %q", body)
		}
	})

	t.Run("evening rain sends", func(t *testing.T) {
		_, body, send := buildMessage(store.SlotEvening, rain)
		if !send {
			t.Fatal("expected shouldSend=true for rain")
		}
		if body != "오늘 퇴근길 비 소식, 우산 챙기세요" {
			t.Errorf("body = %q", body)
		}
	})

	t.Run("dry skips", func(t *testing.T) {
		_, _, send := buildMessage(store.SlotMorning, dry)
		if send {
			t.Error("expected shouldSend=false when no umbrella needed")
		}
	})
}

// TestHasExpoTokenPrefix 는 정식/dev 토큰 구분을 검증한다.
func TestHasExpoTokenPrefix(t *testing.T) {
	cases := []struct {
		token string
		want  bool
	}{
		{"ExponentPushToken[abc123]", true},
		{"dev-token-xyz", false},
		{"", false},
		{"Exponent", false},
	}
	for _, tc := range cases {
		if got := hasExpoTokenPrefix(tc.token); got != tc.want {
			t.Errorf("hasExpoTokenPrefix(%q) = %v; want %v", tc.token, got, tc.want)
		}
	}
}
