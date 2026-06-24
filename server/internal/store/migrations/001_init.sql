-- spec §2 데이터 모델. 로그인 없음 → 기기당 한 줄.
-- 식별자는 Expo 푸시 토큰.

CREATE TABLE IF NOT EXISTS devices (
    push_token             TEXT PRIMARY KEY,          -- Expo 푸시 토큰 (익명 식별자)
    home_nx                INT  NOT NULL,             -- 집 격자 X (기상청 LCC DFS)
    home_ny                INT  NOT NULL,             -- 집 격자 Y
    work_nx                INT  NOT NULL,             -- 회사 격자 X
    work_ny                INT  NOT NULL,             -- 회사 격자 Y
    commute_start          TEXT NOT NULL,             -- 출근 시각 "0900" (HHmm, KST)
    commute_end            TEXT NOT NULL,             -- 퇴근 시각 "1800"
    -- 중복 푸시 방지 (spec §5). devices 컬럼 방식 채택.
    last_morning_push_date TEXT,                      -- "YYYYMMDD"
    last_evening_push_date  TEXT,                     -- "YYYYMMDD"
    last_synced_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- cron이 "지금부터 ~30분 뒤가 출근/퇴근인 기기"를 빠르게 SELECT 하기 위한 인덱스
CREATE INDEX IF NOT EXISTS idx_devices_commute_start ON devices (commute_start);
CREATE INDEX IF NOT EXISTS idx_devices_commute_end   ON devices (commute_end);
