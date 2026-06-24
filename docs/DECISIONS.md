# 설계 결정 & 남은 일

> spec의 "TODO(확정 필요)"들이 어떻게 결정됐는지, 그리고 아직 안 한 일. 큰 그림은 [PROJECT.md](./PROJECT.md).

## 확정된 결정 (spec의 미해결 항목 → 결론)

| 항목 | 결정 | 이유 / 비고 |
|---|---|---|
| **DB** | **Neon** (Supabase 아님) | 무료·무기한, 싱가포르 리전. Render Free Postgres(30일 만료) 회피 |
| **패키지 매니저** | **pnpm** (npm 아님) | `node-linker=hoisted` |
| **단기예보 base_time 마진** | **15분** | 가이드 명시 10분 + 여유 5분. (45분은 초단기용) → [WEATHER-API.md](./WEATHER-API.md) |
| **초단기예보 마진** | 45분 | 매시 30분 발표 |
| **cron 간격 / 리드타임** | tick 10분 / lead 30분 | `PUSH_LEAD_MINUTES` env |
| **중복 푸시 방지** | devices 컬럼 방식 | `last_morning/evening_push_date` (별도 테이블 아님) |
| **강수확률 임계값** | POP ≥ 60% | `popUmbrellaThreshold` |
| **출퇴근 윈도우** | 출근 2전~1후 / 퇴근 1전~2후 (비대칭) | 가는 길/오는 길 중요도 |
| **우산 판정 단위** | **윈도우 전체** (anchor 1점 아님) | 퇴근 정시 맑아도 직후 소나기 포착 → [WEATHER-API.md](./WEATHER-API.md) |
| **메인 화면 모델** | "날짜 섹션" 패러다임, 우산은 아침 한 번 결정 | → [design-main-screen.md](./design-main-screen.md) |
| **시간별 흐름 펼침** | 하단 시트(`HourlySheet`), fade+slide 애니메이션 | 인라인 확장 아님 |
| **주소 검색** | Daum 우편번호를 webview 직접 구현 | @actbase 라이브러리 사파리 탈출 버그로 교체 |
| **푸시 토큰 폴백** | `dev-<deviceId>` | 권한/EAS 부재 시. 토큰은 항상 보장(forecast 호출용) |
| **앱 SDK** | Expo SDK 54 (React 19, RN 0.81) | 실기기 Expo Go(54) 호환 위해 51→54 업그레이드 |
| **네비게이션** | @react-navigation/native-stack | iOS edge swipe 뒤로가기 |
| **GitHub 계정** | 이 리포만 개인 계정(`twkim8548`/`rlaxodnd95@naver.com`) | 다른 리포는 회사 계정. `git config --global` 금지 |

## 푸시 발송 규칙 (확정)

- 출발 **30분 전** 발송. 비 올 때만(`NeedUmbrella` 윈도우 판정). 비 안 오면 조용히 넘어감.
- 같은 슬롯(출근/퇴근)은 하루 1회(`MarkPushed`).
- 정식 `ExponentPushToken[...]`만 발송. dev 토큰은 skip하되 MarkPushed 안 함(나중에 정식 토큰 붙으면 받게).
- 문구: 제목 "우산 챙기세요! ☔️" / 본문 "오늘 {출근길/퇴근길}에 비소식이 있어요".

## 검증 완료 (2026-06)

실기기 푸시 파이프라인 전 구간:
- SDK 51→54 업그레이드 → 실기기 정식 `ExponentPushToken` 발급 → /sync 등록
- `push.Send` → Expo → APNs → 단말 수신 (포그라운드/백그라운드/잠금 전부)
- cron 전 구간: DueDevices 매칭 → 예보 조회 → 조건부 발송 → MarkPushed → 중복방지(재실행 due=0)

## 구현 완성도

### 완료 (spec §9 구현 순서 1~6)
- [x] DB + `/sync` (격자 변환, upsert)
- [x] `/forecast` (캐싱·재시도·초단기 혼합·4시점·윈도우 판정·근거)
- [x] RN 앱 골격 (설정/로컬 저장/sync)
- [x] 메인 화면 (날짜 섹션·시간별 시트·당겨서 새로고침·날씨 아이콘·근거 표시)
- [x] Expo Push 셋업 (정식 토큰 + dev 폴백)
- [x] Cron 푸시 (윈도우 판정·중복방지)
- [x] 네비게이션(edge swipe), 권한 프라이밍

### 미구현 / 보류 (§9-7 고도화)
- [ ] `GET /forecast/now` — 501 (선택 기능)
- [ ] **어제 대비 체감** — 전일 동시간 기온 비교, "어제보다 추워요". cron에 TODO 주석, 타입(`feelsVsYesterday`)만 존재
- [ ] 체감 기반 발송 — 우산 무관하게 옷차림 변화 크면 발송 (cron §9-7 TODO)
- [ ] 미세먼지 (에어코리아, 별도 활용신청)
- [ ] LLM 한 줄 메시지 생성

## 운영 전 반드시 (Production 체크리스트)

- [ ] **Neon 비밀번호 로테이션** — 개발 중 노출됨(채팅/`.env`). 배포 전 교체
- [ ] **EAS 정식 빌드** — 현재 Expo Go로 테스트. standalone 빌드라야 사용자가 앱 안 켜도 푸시 수신(진짜 배포 형태)
- [ ] `app.json` `extra.apiBaseUrl`을 LAN IP → **운영 서버 URL**로
- [ ] weather 클라이언트 재시도/폴백 운영 환경에서 재검증
- [ ] Render 웹/cron 서비스 배포 + 환경변수 설정
- [ ] Render Trial 계정은 GitHub 연결로 검증해야 아웃바운드 네트워크 열림(spec §6)

## 환경변수 정리

**서버 api**: `DATABASE_URL`(필수), `KMA_SERVICE_KEY`(필수), `KAKAO_REST_API_KEY`(필수), `PORT`(8080), `KMA_BASE_URL`, `EXPO_PUSH_URL`
**cron**: `DATABASE_URL`, `KMA_SERVICE_KEY`(필수), `PUSH_LEAD_MINUTES`(30), 테스트용 `CRON_NOW`/`CRON_FORCE_SEND`
**앱(app.json extra)**: `apiBaseUrl`, `eas.projectId`

모든 비밀키는 `server/.env`(gitignored). git에 올리지 말 것.
