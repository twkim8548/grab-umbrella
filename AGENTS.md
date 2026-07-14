# AGENTS.md — grab-umbrella

이 레포에서 작업할 때 따를 규칙·컨텍스트. 세부는 `docs/`로 연결한다.

## 이 프로젝트가 뭔지

출퇴근길 날씨 알림 앱 "우산챙겨?". 출근/퇴근 두 시점의 날씨에만 집중하고, 킬러 기능은 **푸시**(앱 안 켜도 출발 30분 전 "우산 챙기세요"). 모노레포: `server/`(Go) + `app/`(React Native/Expo).

→ 전체 개요는 **[docs/PROJECT.md](./docs/PROJECT.md)** 부터 읽을 것. 구현 상세 [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md), 날씨 로직 [docs/WEATHER-API.md](./docs/WEATHER-API.md), 설계 결정 [docs/DECISIONS.md](./docs/DECISIONS.md).

## 반드시 지킬 규칙

- **git 계정: 이 레포만 개인 계정.** author `rlaxodnd95@naver.com`, GitHub `twkim8548`. 다른 레포는 회사 계정. ⚠️ **`git config --global`로 개인 이메일을 설정하지 말 것** — 로컬 설정(`git config user.email`)만 사용. 커밋/푸시는 사용자가 요청할 때만.
- **패키지 매니저: pnpm** (npm 아님). `app/`에서 `pnpm install`, `node-linker=hoisted`.
- **DB: Neon** (Supabase 아님). Postgres, pgx v5.
- **비밀키는 `server/.env`(gitignored)에만.** 값(DB URL, KMA/Kakao 키)을 코드·문서·커밋에 넣지 말 것. 위치만 참조.
- **내부 노트는 git에 안 올림.** 남은 작업·운영 메모는 `*.local.md`(예 `docs/TODO.local.md`)나 `docs/internal/`에. 이미 gitignore됨.

## 자주 헷갈리는 실제값 (spec과 다름)

- 앱: **Expo SDK 54** (React 19, RN 0.81). 서버: **Go 1.22**, 포트 8080.
- 단기예보 base_time 마진 **15분** (45분 아님 — 45분은 초단기용).
- 우산 판정 = **출퇴근 윈도우 전체** 기준 (anchor 1점 아님). `WindowNeedUmbrella`. → [docs/WEATHER-API.md](./docs/WEATHER-API.md).
- `/forecast`는 오늘·내일 **4시점** 반환, 지난 시점은 null.

## 작업 관례

- 서버 변경 후: `cd server && go build ./... && go vet ./... && go test ./...`
- 앱 변경 후: `cd app && npx tsc --noEmit`
- 실기기 테스트: Expo Go(SDK 54), `app.json` `extra.apiBaseUrl`을 Mac LAN IP로.
- **UI 시나리오 확인**: `app.json` `extra.mockForecast` 값 변경 (`sunny/cloudy/rain/shower/shower-later/none`, 4시점 콤마). 빈 값 = 실제 데이터.
- 개발 도구: `cmd/peek`(기기 조회), `cmd/diag`(예보 점검), `cmd/testpush`(실발송). cron 테스트 훅: `CRON_NOW`, `CRON_FORCE_SEND`(운영 미설정).
