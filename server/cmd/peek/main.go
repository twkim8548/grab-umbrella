// Command peek 은 개발용 일회성 조회 도구다. devices 테이블의 등록 상태를 본다.
// 운영 배포 대상 아님(psql 부재 환경에서 토큰/슬롯 상태 확인용).
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Fatal("DATABASE_URL required")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	rows, err := pool.Query(context.Background(), `
		SELECT push_token, home_address, work_address,
		       commute_start, commute_end,
		       COALESCE(last_morning_push_date,''), COALESCE(last_evening_push_date,''),
		       last_synced_at
		FROM devices ORDER BY last_synced_at DESC`)
	if err != nil {
		log.Fatalf("query: %v", err)
	}
	defer rows.Close()

	n := 0
	for rows.Next() {
		var tok, ha, wa, cs, ce, lm, le string
		var upd any
		if err := rows.Scan(&tok, &ha, &wa, &cs, &ce, &lm, &le, &upd); err != nil {
			log.Fatalf("scan: %v", err)
		}
		n++
		kind := "dev"
		if len(tok) >= 18 && tok[:18] == "ExponentPushToken[" {
			kind = "EXPO ✅"
		}
		mask := tok
		if len(mask) > 34 {
			mask = mask[:34] + "…"
		}
		fmt.Printf("[%d] %s  token=%s\n    home=%q work=%q commute=%s~%s  pushed(m/e)=%q/%q  updated=%v\n",
			n, kind, mask, ha, wa, cs, ce, lm, le, upd)
	}
	if err := rows.Err(); err != nil {
		log.Fatalf("rows: %v", err)
	}
	fmt.Printf("--- %d device(s) ---\n", n)
}
