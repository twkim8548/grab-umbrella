package main

import (
	"testing"

	"github.com/twkim8548/grab-umbrella/server/internal/store"
)

// TestBuildMessage 는 메시지 압축 규칙을 검증한다(spec §5): 우산 필요할 때만 발송.
// 우산 판정(윈도우 OR)은 호출부에서 끝나고, buildMessage 는 bool 만 받는다.
func TestBuildMessage(t *testing.T) {
	t.Run("morning rain sends", func(t *testing.T) {
		title, body, send := buildMessage(store.SlotMorning, true)
		if !send {
			t.Fatal("expected shouldSend=true for rain")
		}
		if title != "우산 챙기세요! ☔️" {
			t.Errorf("title = %q", title)
		}
		if body != "오늘 출근길에 비소식이 있어요" {
			t.Errorf("body = %q", body)
		}
	})

	t.Run("evening rain sends", func(t *testing.T) {
		_, body, send := buildMessage(store.SlotEvening, true)
		if !send {
			t.Fatal("expected shouldSend=true for rain")
		}
		if body != "오늘 퇴근길에 비소식이 있어요" {
			t.Errorf("body = %q", body)
		}
	})

	t.Run("dry skips", func(t *testing.T) {
		_, _, send := buildMessage(store.SlotMorning, false)
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
