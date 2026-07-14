-- 앱 내 알림 스위치. 기존 기기와 필드를 보내지 않는 구버전 앱은 활성 상태를 유지한다.
ALTER TABLE devices
    ADD COLUMN IF NOT EXISTS notifications_enabled BOOLEAN NOT NULL DEFAULT TRUE;
