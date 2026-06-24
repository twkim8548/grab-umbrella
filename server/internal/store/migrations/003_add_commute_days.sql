-- spec 추가: 출근일(요일) 설정. 7자리 "일월화수목금토", 1=on. 이 요일에만 푸시 발송.
-- 기존 기기는 평일(월~금)="0111110" 으로 기본 설정. NOT NULL + DEFAULT 로 안전하게 추가.
ALTER TABLE devices ADD COLUMN IF NOT EXISTS commute_days TEXT NOT NULL DEFAULT '0111110';
