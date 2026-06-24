package store

import (
	"context"
	"embed"
	"fmt"
	"sort"
)

// migrations/ 의 .sql 을 바이너리에 embed 한다. 파일 경로 의존성이 없어 어디서 실행하든
// (로컬·Docker·Railway) 동일하게 동작한다. 마이그레이션은 모두 idempotent(IF NOT EXISTS)라
// 매 기동 시 재실행해도 안전하다.
//
//go:embed migrations/*.sql
var migrationFiles embed.FS

// Migrate 는 embed 된 migrations 를 파일명 순서대로 적용한다. 추가형(IF NOT EXISTS) SQL 이라
// 여러 번 실행해도 기존 데이터에 영향이 없다. api 기동 시 1회 호출해 스키마를 보장한다.
func (s *Store) Migrate(ctx context.Context) error {
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names) // 001_, 002_ … 순서 보장
	if len(names) == 0 {
		return fmt.Errorf("no migration files embedded")
	}

	for _, name := range names {
		sql, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		if _, err := s.pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
	}
	return nil
}
