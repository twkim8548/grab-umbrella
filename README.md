# 우산챙겨? (grab-umbrella)

출퇴근길 날씨 알림 앱. 사용자가 실제로 바깥에 있는 두 순간 — **출근 시각, 퇴근 시각** — 의 날씨에만 집중한다.

앱이 답하는 질문은 단 두 개:
- 오늘 출근/퇴근할 때 우산 챙겨야 해?
- 옷은 어떻게 입어야 해? (어제 대비 체감 — 고도화 예정)

킬러 기능은 화면이 아니라 **푸시**다. 앱을 안 켜도 출발 30분 전에 "우산 챙기세요" 한 줄이 온다.

## 문서 (docs/)

| 문서 | 내용 |
|---|---|
| [docs/PROJECT.md](./docs/PROJECT.md) | **여기부터** — 개요·컨셉·스택·구조 (진입점) |
| [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md) | 서버/앱 실제 구현 상세 (API·DB·컴포넌트) |
| [docs/WEATHER-API.md](./docs/WEATHER-API.md) | 기상청 API·base_time 마진·우산 판정 로직 |
| [docs/DECISIONS.md](./docs/DECISIONS.md) | 확정된 설계 결정 (왜 그렇게 정했는지) |
| [docs/design-main-screen.md](./docs/design-main-screen.md) | 메인 화면 디자인 확정안 |
| [docs/kma-guide/](./docs/kma-guide/README.md) | 기상청 공식 활용가이드 (출처) |

## 모노레포 구조

```
grab-umbrella/
├── server/    # Go 백엔드 (cmd: api/cron/migrate + 개발도구)
├── app/       # React Native (Expo SDK 54) 앱
└── docs/      # 설계·구현 문서
```

## 기술 스택

| 영역 | 선택 |
|---|---|
| 앱 | React Native + Expo SDK 54 (React 19, RN 0.81) |
| 서버 | Go 1.22 + chi |
| DB | **Neon** (Postgres, 싱가포르) |
| 호스팅 | Render (Free) — 웹 + Cron 별도 |
| 푸시 | Expo Push |

**로그인 없음.** 기기 기반 식별 — 식별자는 Expo 푸시 토큰.

## 빠른 시작

### 서버
```bash
cd server
# .env 에 DATABASE_URL, KMA_SERVICE_KEY, KAKAO_REST_API_KEY 채우기 (gitignored)
go run ./cmd/migrate   # 최초 1회
go run ./cmd/api       # :8080
```

### 앱
```bash
cd app
pnpm install
pnpm expo start --lan  # 실기기는 app.json extra.apiBaseUrl 의 LAN IP 사용
```

자세한 구조·설계 결정은 [docs/PROJECT.md](./docs/PROJECT.md), [docs/DECISIONS.md](./docs/DECISIONS.md) 참고.
