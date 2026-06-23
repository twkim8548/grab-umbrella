// Command migrate 는 migrations/ 의 .sql 파일을 순서대로 적용한다.
// 의존성 없이 단순 실행 — 작은 프로젝트라 전용 마이그레이션 툴은 과함.
//
// 사용: DATABASE_URL=... go run ./cmd/migrate
package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("missing DATABASE_URL")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("ping: %v", err)
	}
	log.Println("connected ✓")

	files, err := filepath.Glob("migrations/*.sql")
	if err != nil {
		log.Fatalf("glob: %v", err)
	}
	sort.Strings(files)
	if len(files) == 0 {
		log.Fatal("no migration files found (run from server/ dir)")
	}

	for _, f := range files {
		sql, err := os.ReadFile(f)
		if err != nil {
			log.Fatalf("read %s: %v", f, err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			log.Fatalf("apply %s: %v", f, err)
		}
		log.Printf("applied %s ✓", f)
	}
	log.Println("all migrations applied ✓")
}
