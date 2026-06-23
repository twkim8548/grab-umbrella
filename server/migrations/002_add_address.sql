-- spec §2: /sync 가 주소 기반으로 바뀌면서 표시 보조용 주소 원문을 저장한다.
-- 격자(home_nx/ny, work_nx/ny)가 여전히 실제 위치 식별자이고, 주소는 앱 설정화면
-- 재표시용 보조 데이터다(설정의 주인은 앱 로컬). nullable 로 추가.
ALTER TABLE devices ADD COLUMN IF NOT EXISTS home_address TEXT;
ALTER TABLE devices ADD COLUMN IF NOT EXISTS work_address TEXT;
