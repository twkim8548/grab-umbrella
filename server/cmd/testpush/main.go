// Command testpush 는 개발용 일회성 도구다. 인자로 받은 토큰(또는 DB 의 최신 정식
// Expo 토큰)으로 실제 푸시를 한 발 발송해 단말 수신을 검증한다. 운영 배포 대상 아님.
//
// 사용:
//   go run ./cmd/testpush                 # DB 최신 ExponentPushToken 으로 발송
//   go run ./cmd/testpush "ExponentPushToken[...]"  # 지정 토큰으로 발송
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twkim8548/grab-umbrella/server/internal/push"
)

func main() {
	ctx := context.Background()

	to := ""
	if len(os.Args) > 1 {
		to = os.Args[1]
	} else {
		to = latestExpoToken(ctx)
	}
	if !strings.HasPrefix(to, "ExponentPushToken[") {
		log.Fatalf("정식 Expo 토큰이 아님: %q", to)
	}

	pc := push.New(env("EXPO_PUSH_URL", "https://exp.host/--/api/v2/push/send"))
	// cron 의 실제 발송 문구(출근 슬롯)와 동일하게 맞춰 단말 표시를 검증한다.
	msg := push.Message{
		To:    to,
		Title: "우산 챙기세요!",
		Body:  "오늘 출근길에 비소식이 있어요",
	}
	fmt.Printf("발송 → %s…\n", to[:34])
	if err := pc.Send(ctx, msg); err != nil {
		log.Fatalf("발송 실패: %v", err)
	}
	fmt.Println("발송 성공 (Expo 가 ok 응답). 단말 알림을 확인하세요.")
}

func latestExpoToken(ctx context.Context) string {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Fatal("DATABASE_URL required")
	}
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	var tok string
	err = pool.QueryRow(ctx, `
		SELECT push_token FROM devices
		WHERE push_token LIKE 'ExponentPushToken[%'
		ORDER BY last_synced_at DESC LIMIT 1`).Scan(&tok)
	if err != nil {
		log.Fatalf("토큰 조회 실패: %v", err)
	}
	return tok
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
