// Command migrate 는 마이그레이션을 수동으로 적용한다(배포와 별개로 직접 돌리고 싶을 때).
// 실제 마이그레이션 로직·SQL 은 internal/store 에 embed 되어 있어, api 기동 시 자동 적용과
// 동일한 코드를 쓴다. 어디서 실행하든(파일 경로 무관) 동작한다.
//
// 사용: DATABASE_URL=... go run ./cmd/migrate
package main

import (
	"context"
	"log"
	"os"

	"github.com/twkim8548/grab-umbrella/server/internal/store"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("missing DATABASE_URL")
	}

	ctx := context.Background()
	st, err := store.New(ctx, dsn)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer st.Close()

	if err := st.Migrate(ctx); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	log.Println("all migrations applied ✓")
}
