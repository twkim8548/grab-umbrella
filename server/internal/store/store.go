// Package store 는 devices 테이블 접근을 담당한다. spec §2.
package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Device 는 devices 테이블 한 줄에 대응한다.
type Device struct {
	PushToken           string
	HomeNx, HomeNy      int
	WorkNx, WorkNy      int
	HomeAddress         string // 표시 보조용 원문 주소 (nullable)
	WorkAddress         string
	CommuteStart        string // "0900"
	CommuteEnd          string // "1800"
	LastMorningPushDate string // "YYYYMMDD"
	LastEveningPushDate string
	LastSyncedAt        time.Time
	CreatedAt           time.Time
}

type Store struct {
	pool *pgxpool.Pool
}

// New 는 DATABASE_URL 로 풀을 연다.
func New(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

// Upsert 는 /sync 에서 호출. 동기화는 항상 로컬 → DB 단방향 (spec §2).
func (s *Store) Upsert(ctx context.Context, d Device) error {
	const q = `
INSERT INTO devices (push_token, home_nx, home_ny, work_nx, work_ny,
                     home_address, work_address,
                     commute_start, commute_end, last_synced_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9, now())
ON CONFLICT (push_token) DO UPDATE SET
    home_nx = EXCLUDED.home_nx,
    home_ny = EXCLUDED.home_ny,
    work_nx = EXCLUDED.work_nx,
    work_ny = EXCLUDED.work_ny,
    home_address = EXCLUDED.home_address,
    work_address = EXCLUDED.work_address,
    commute_start = EXCLUDED.commute_start,
    commute_end = EXCLUDED.commute_end,
    last_synced_at = now();`
	_, err := s.pool.Exec(ctx, q, d.PushToken, d.HomeNx, d.HomeNy,
		d.WorkNx, d.WorkNy, d.HomeAddress, d.WorkAddress,
		d.CommuteStart, d.CommuteEnd)
	return err
}

// GetByToken 은 /forecast 에서 push_token 으로 위치·시각 조회.
func (s *Store) GetByToken(ctx context.Context, token string) (*Device, error) {
	const q = `SELECT push_token, home_nx, home_ny, work_nx, work_ny,
	                  commute_start, commute_end
	           FROM devices WHERE push_token = $1;`
	var d Device
	err := s.pool.QueryRow(ctx, q, token).Scan(
		&d.PushToken, &d.HomeNx, &d.HomeNy, &d.WorkNx, &d.WorkNy,
		&d.CommuteStart, &d.CommuteEnd)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// TODO(cron): "지금부터 ~lead분 뒤가 출근/퇴근인 기기" SELECT 메서드 (spec §5).
