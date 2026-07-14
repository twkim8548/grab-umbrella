// Package store 는 devices 테이블 접근을 담당한다. spec §2.
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Device 는 devices 테이블 한 줄에 대응한다.
type Device struct {
	PushToken            string
	HomeNx, HomeNy       int
	WorkNx, WorkNy       int
	HomeAddress          string // 표시 보조용 원문 주소 (nullable)
	WorkAddress          string
	CommuteStart         string // "0900"
	CommuteEnd           string // "1800"
	CommuteDays          string // "0111110" 일~토, 1=on. 이 요일에만 발송.
	NotificationsEnabled bool   // 앱 내 알림 스위치. false 면 cron 발송 대상에서 제외.
	LastMorningPushDate  string // "YYYYMMDD"
	LastEveningPushDate  string
	LastSyncedAt         time.Time
	CreatedAt            time.Time
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
                     commute_start, commute_end, commute_days, notifications_enabled,
                     last_synced_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11, now())
ON CONFLICT (push_token) DO UPDATE SET
    home_nx = EXCLUDED.home_nx,
    home_ny = EXCLUDED.home_ny,
    work_nx = EXCLUDED.work_nx,
    work_ny = EXCLUDED.work_ny,
    home_address = EXCLUDED.home_address,
    work_address = EXCLUDED.work_address,
    commute_start = EXCLUDED.commute_start,
    commute_end = EXCLUDED.commute_end,
    commute_days = EXCLUDED.commute_days,
    notifications_enabled = EXCLUDED.notifications_enabled,
    last_synced_at = now();`
	_, err := s.pool.Exec(ctx, q, d.PushToken, d.HomeNx, d.HomeNy,
		d.WorkNx, d.WorkNy, d.HomeAddress, d.WorkAddress,
		d.CommuteStart, d.CommuteEnd, d.CommuteDays, d.NotificationsEnabled)
	return err
}

// GetByToken 은 /forecast 와 /sync 에서 push_token 으로 위치·시각·주소 조회.
func (s *Store) GetByToken(ctx context.Context, token string) (*Device, error) {
	const q = `SELECT push_token, home_nx, home_ny, work_nx, work_ny,
	                  COALESCE(home_address, ''), COALESCE(work_address, ''),
	                  commute_start, commute_end, notifications_enabled
	           FROM devices WHERE push_token = $1;`
	var d Device
	err := s.pool.QueryRow(ctx, q, token).Scan(
		&d.PushToken, &d.HomeNx, &d.HomeNy, &d.WorkNx, &d.WorkNy,
		&d.HomeAddress, &d.WorkAddress,
		&d.CommuteStart, &d.CommuteEnd, &d.NotificationsEnabled)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// kst 는 한국 표준시(Asia/Seoul, UTC+9) 고정 오프셋이다. weather 패키지와 동일하게
// tzdata 없는 환경에서도 동작하도록 FixedZone 을 쓴다. cron 의 시각 비교는 KST 기준이다.
var kst = time.FixedZone("KST", 9*60*60)

// 슬롯 식별자. cron 이 어느 격자/시각으로 예보를 조회할지 결정하는 데 쓴다.
const (
	SlotMorning = "morning"
	SlotEvening = "evening"
)

// tickInterval 은 cron 실행 간격(기본 10분)이다. "now+lead 가 commute 시각의 이 구간에
// 드는가" 윈도우 매칭에 쓴다. cron 주기를 바꾸면 이 값도 맞춰야 정확히 한 번 걸린다.
const tickInterval = 10 * time.Minute

// DueDevice 는 지금 푸시를 보내야 하는 기기 한 건이다. cron 이 바로 쓸 수 있게
// 어느 슬롯(출근/퇴근)·어느 격자·어느 시각으로 예보를 조회할지 함께 담는다.
type DueDevice struct {
	Device
	Slot     string // "morning" | "evening"
	Nx, Ny   int    // 예보 조회 격자 (morning=집, evening=회사)
	FcstTime string // 예보 슬롯 시각 "HHmm" (commute 시각)
}

// dueSlot 은 순수 매칭 로직이다(테스트 가능, DB 불필요).
// now+lead 의 타겟시각이 commute_start/end 와 같은 tick 구간에 들면 해당 슬롯을 반환한다.
// 윈도우: commute 시각이 [target, target+tick) 에 들면 due. 둘 다면 morning 우선.
// 일치 없으면 "" 반환. commute 형식이 "HHmm" 아니면 무시한다.
func dueSlot(now time.Time, leadMinutes int, tick time.Duration, commuteStart, commuteEnd string) string {
	n := now.In(kst)
	target := n.Add(time.Duration(leadMinutes) * time.Minute)
	targetMin := target.Hour()*60 + target.Minute()
	windowMin := int(tick / time.Minute)

	inWindow := func(commute string) bool {
		cm, ok := hhmmToMinutes(commute)
		if !ok {
			return false
		}
		// commute ∈ [target, target+window). 자정 경계를 넘는 비교는 단순화를 위해
		// 분(分) 단위 일치 윈도우로만 본다(commute 와 target 모두 같은 날 HHmm 가정).
		return cm >= targetMin && cm < targetMin+windowMin
	}

	if inWindow(commuteStart) {
		return SlotMorning
	}
	if inWindow(commuteEnd) {
		return SlotEvening
	}
	return ""
}

// dayOn 은 commute_days("일월화수목금토" 7자리, 1=on)에서 weekday(0=일…6=토)가 켜졌는지 본다.
// 형식이 7자리가 아니면(구버전/손상) 평일 기준으로 폴백한다(월~금 on).
func dayOn(days string, weekday int) bool {
	if len(days) != 7 {
		return weekday >= 1 && weekday <= 5 // 폴백: 월~금
	}
	if weekday < 0 || weekday > 6 {
		return false
	}
	return days[weekday] == '1'
}

// hhmmToMinutes 는 "HHmm" 을 자정 기준 분(分)으로 바꾼다. 형식이 아니면 ok=false.
func hhmmToMinutes(s string) (int, bool) {
	if len(s) != 4 {
		return 0, false
	}
	for i := 0; i < 4; i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
	}
	h := int(s[0]-'0')*10 + int(s[1]-'0')
	m := int(s[2]-'0')*10 + int(s[3]-'0')
	if h > 23 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

// DueDevices 는 "지금부터 leadMinutes 뒤가 출근 또는 퇴근 시각"인 기기를 찾아
// 슬롯·격자·예보시각과 함께 반환한다(spec §5). 중복 방지: 해당 슬롯의
// last_*_push_date 가 오늘(YYYYMMDD)과 같으면 SQL 에서 제외한다.
//
// 매칭은 dueSlot 윈도우 규칙(now+lead 가 commute 시각의 같은 tick 구간)으로 한다.
// SQL 은 후보를 좁히기만 하고(이미 발송한 건 제외), 최종 슬롯 판정은 Go 에서 한다.
func (s *Store) DueDevices(ctx context.Context, now time.Time, leadMinutes int) ([]DueDevice, error) {
	today := now.In(kst).Format("20060102")

	// 아직 오늘 morning/evening 둘 다 발송한 기기는 후보에서 제외한다.
	const q = `SELECT push_token, home_nx, home_ny, work_nx, work_ny,
	                  commute_start, commute_end, commute_days, notifications_enabled,
	                  COALESCE(last_morning_push_date, ''), COALESCE(last_evening_push_date, '')
	           FROM devices
	           WHERE notifications_enabled
	             AND (last_morning_push_date IS DISTINCT FROM $1
	               OR last_evening_push_date IS DISTINCT FROM $1);`

	rows, err := s.pool.Query(ctx, q, today)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 오늘 요일 인덱스(0=일 … 6=토). commute_days 의 해당 자리가 "1" 이어야 발송 대상.
	weekday := int(now.In(kst).Weekday())

	var due []DueDevice
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.PushToken, &d.HomeNx, &d.HomeNy, &d.WorkNx, &d.WorkNy,
			&d.CommuteStart, &d.CommuteEnd, &d.CommuteDays, &d.NotificationsEnabled,
			&d.LastMorningPushDate, &d.LastEveningPushDate); err != nil {
			return nil, err
		}

		// 출근일이 아닌 요일은 출/퇴근 모두 건너뛴다(spec: 선택한 출근일에만 발송).
		if !dayOn(d.CommuteDays, weekday) {
			continue
		}

		slot := dueSlot(now, leadMinutes, tickInterval, d.CommuteStart, d.CommuteEnd)
		switch slot {
		case SlotMorning:
			if d.LastMorningPushDate == today {
				continue // 오늘 출근건 이미 발송.
			}
			due = append(due, DueDevice{Device: d, Slot: slot,
				Nx: d.HomeNx, Ny: d.HomeNy, FcstTime: d.CommuteStart})
		case SlotEvening:
			if d.LastEveningPushDate == today {
				continue // 오늘 퇴근건 이미 발송.
			}
			due = append(due, DueDevice{Device: d, Slot: slot,
				Nx: d.WorkNx, Ny: d.WorkNy, FcstTime: d.CommuteEnd})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return due, nil
}

// MarkPushed 는 중복 방지용으로 해당 슬롯의 발송 날짜(YYYYMMDD)를 기록한다(spec §5).
func (s *Store) MarkPushed(ctx context.Context, pushToken, slot, date string) error {
	var col string
	switch slot {
	case SlotMorning:
		col = "last_morning_push_date"
	case SlotEvening:
		col = "last_evening_push_date"
	default:
		return fmt.Errorf("store: unknown slot %q", slot)
	}
	q := "UPDATE devices SET " + col + " = $1 WHERE push_token = $2;"
	_, err := s.pool.Exec(ctx, q, date, pushToken)
	return err
}
