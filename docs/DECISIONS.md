# 설계 결정

> spec의 "TODO(확정 필요)"들이 어떻게 결정됐는지. 큰 그림은 [PROJECT.md](./PROJECT.md).

## 확정된 결정 (spec의 미해결 항목 → 결론)

| 항목 | 결정 | 이유 / 비고 |
|---|---|---|
| **DB** | **Neon** (Supabase 아님) | 무료·무기한, 싱가포르 리전. Render Free Postgres(30일 만료) 회피 |
| **운영 런타임** | AWS Lambda Function URL + EventBridge Scheduler | API는 Web Adapter, cron은 10분마다 Lambda 직접 호출. SAM으로 함께 관리 |
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
| **앱 예보 캐시** | KST 당일·3시간 TTL + stale-while-refresh | 오래된 날짜·변경된 위치/시각은 폐기하고, 유효 캐시는 즉시 표시 후 최신값으로 교체 |
| **요청 수명 관리** | 앱 15초 제한 + 화면별 취소, 서버 조립 12초 제한 | 느린 외부 API가 홈을 무기한 막거나 이전 응답이 새 화면을 덮는 문제 방지 |
| **푸시 토큰 발급** | 세션 캐시 + 동시 요청 병합 | 화면 focus마다 불필요한 토큰 발급을 피하고 권한 변경 때만 갱신 |
| **주소 지오코딩** | 기존 주소와 같은 쪽은 저장 격자 재사용 | 알림·시간·요일만 바꾼 저장에서 불필요한 카카오 API 호출 제거 |
| **GitHub 계정** | 이 리포만 개인 계정(`twkim8548`/`rlaxodnd95@naver.com`) | 다른 리포는 회사 계정. `git config --global` 금지 |

## 푸시 발송 규칙 (확정)

- 출발 **30분 전** 발송. 비 올 때만(`NeedUmbrella` 윈도우 판정). 비 안 오면 조용히 넘어감.
- 앱의 `알림 받기`가 꺼져 있으면 서버 `notifications_enabled=false`로 동기화하고 cron 대상에서 제외.
- 같은 슬롯(출근/퇴근)은 하루 1회(`MarkPushed`).
- 정식 `ExponentPushToken[...]`만 발송. dev 토큰은 skip하되 MarkPushed 안 함(나중에 정식 토큰 붙으면 받게).
- 문구: 제목 "우산 챙기세요! ☔️" / 본문 "오늘 {출근길/퇴근길}에 비소식이 있어요".

## 검증 완료 (2026-07)

Expo Push capability가 있는 실기기 테스트 경로에서 푸시 파이프라인 전 구간:
- SDK 51→54 업그레이드 → 실기기 정식 `ExponentPushToken` 발급 → /sync 등록
- `push.Send` → Expo → APNs → 단말 수신 (포그라운드/백그라운드/잠금 전부)
- cron 전 구간: DueDevices 매칭 → 예보 조회 → 조건부 발송 → MarkPushed → 중복방지(재실행 due=0)

iPhone Release 빌드 통합 시나리오:

- 앱 삭제 후 첫 실행·권한 프라이밍·초기 설정·홈 진입
- 알림 스위치 변경과 서버 동기화, 설정 저장 후 홈 복귀
- 유효 캐시 즉시 표시 후 최신 예보 교체, 당겨서 새로고침
- 오프라인 갱신 실패 시 기존 예보 유지·오류 안내·네트워크 복구 후 재시도
- 설정 화면 왕복과 연속 요청에서 이전 요청 취소·늦은 응답 폐기

운영 API와 cron은 AWS Lambda에 배포되어 있다. 현재 iPhone Release 검증본은 무료 개발용 서명으로 직접 설치해 푸시를 제외한 앱 흐름을 확인했다. 이 서명에는 운영 APNs capability가 없으므로, 유료 계정의 TestFlight 빌드에서 푸시 전 구간을 다시 검증해야 한다. App Store/TestFlight 배포는 아직 하지 않았다.

## 구현 상태

핵심 흐름(spec §9 구현 순서 1~6) 완료:
- DB + `/sync` (격자 변환, upsert)
- `/forecast` (캐싱·재시도·초단기 혼합·4시점·윈도우 판정·근거)
- RN 앱 (설정/로컬 저장/sync, 날짜 섹션·시간별 시트·당겨서 새로고침·날씨 아이콘·근거 표시)
- Expo Push (정식 토큰 + dev 폴백), Cron 푸시 (윈도우 판정·중복방지)
- 네비게이션(edge swipe), 권한 프라이밍
- 초기 설정 흐름 안정화, 알림 스위치 서버 발송 설정 연동
- 홈 예보 3시간/KST 캐시, 백그라운드 갱신, 15초 제한·취소·재시도 UX
- 푸시 토큰 세션 캐시·동시 발급 병합, 동일 주소 지오코딩 생략
- AWS SAM 운영 배포와 iOS Release 실기기 통합 검증

§9-7 고도화(어제 대비 체감 등)는 코드 TODO 주석으로 위치를 표시해 두었다.

## 환경변수 정리

**서버 api**: `DATABASE_URL`(필수), `KMA_SERVICE_KEY`(필수), `KAKAO_REST_API_KEY`(필수), `PORT`(8080), `KMA_BASE_URL`, `EXPO_PUSH_URL`
**cron**: `DATABASE_URL`, `KMA_SERVICE_KEY`(필수), `PUSH_LEAD_MINUTES`(30), 테스트용 `CRON_NOW`/`CRON_FORCE_SEND`
**앱(app.json extra)**: `apiBaseUrl`, `eas.projectId`

모든 비밀키는 `server/.env`(gitignored). git에 올리지 말 것.
