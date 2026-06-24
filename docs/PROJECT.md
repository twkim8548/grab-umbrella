# 우산챙겨? (grab-umbrella) — 프로젝트 개요

> **AI 핸드오프용 마스터 문서.** 이 프로젝트를 처음 보는 사람(또는 AI)이 가장 먼저 읽는 곳.
> 세부는 같은 `docs/` 폴더의 다른 문서로 연결한다. 내용은 **실제 코드 기준**(설계 초안 아님).

## 한 줄 정의

출퇴근길 날씨 알림 앱. 사용자가 실제로 바깥에 있는 두 순간 — **출근 시각, 퇴근 시각** — 의 날씨에만 집중한다. "하루 날씨"가 아니라 그 두 시점.

앱이 답하는 질문은 둘:
- 오늘 출근/퇴근할 때 **우산 챙겨야 해?**
- (고도화 예정) 옷은 어떻게 입어야 해? — 어제 대비 체감

**킬러 기능은 화면이 아니라 푸시다.** 앱을 안 켜도 출발 30분 전에 "우산 챙기세요" 한 줄이 온다.

## 이름 / 식별자

| 구분 | 값 |
|---|---|
| 표시 이름 | 우산챙겨? |
| 코드네임 | `grab-umbrella` (리포·폴더·slug) |
| iOS 번들 ID | `com.twkim8548.grabumbrella` |
| Android 패키지 | `com.twkim8548.grabumbrella` |
| EAS projectId | `f43a251c-5a7f-4a51-b198-591f9e766cdc` (owner `twkim8548`) |
| GitHub | `twkim8548/grab-umbrella` (개인 계정, public) |

> 이 리포는 **개인 계정 전용**이다. git author = `rlaxodnd95@naver.com`, 다른 리포는 회사 계정. `git config --global`로 개인 이메일을 설정하지 말 것(로컬 설정만).

## 핵심 컨셉 (왜 이렇게 만드는가)

- **로그인 없음.** 계정이 아니라 **기기 기반**. 식별자는 Expo 푸시 토큰. 서버엔 "누구"가 없고 "이 토큰을 가진 익명 기기"만 있다.
- **설정의 주인은 앱 로컬**(AsyncStorage). DB는 푸시 발송용 사본. 동기화는 항상 **로컬 → DB 단방향**(`/sync`). 충돌 고민 없음. 트레이드오프: 기기를 바꾸면 설정이 안 따라옴(로그인을 뺀 대가).
- **우산은 아침에 한 번 결정된다.** 출근 OR 퇴근, 하나라도 비 → "우산 챙기세요". 자세한 화면 모델은 [design-main-screen.md](./design-main-screen.md).
- **멀리 보는 건 단기예보, 임박한 건 초단기예보.** 메인 화면은 단기예보(글피까지), 푸시는 초단기예보(+6h). 6시간 이내 시점은 초단기, 그 외 단기로 혼합.

## 기술 스택 (확정)

| 영역 | 선택 | 비고 |
|---|---|---|
| 앱 | React Native + **Expo SDK 54** | React 19, RN 0.81 |
| 네비게이션 | @react-navigation/native-stack | iOS edge swipe 뒤로가기 |
| 서버 | **Go 1.22** + chi 라우터 | 단일 바이너리, net/http 기반 |
| DB | **Neon** (Postgres, 싱가포르 리전) | Supabase 아님 — 확정. pgx v5 |
| 호스팅 | Render (Free) | 웹 + Cron 별도 서비스 |
| 스케줄링 | Render Cron Job | 앱의 심장 (출발 30분 전 푸시) |
| 푸시 | Expo Push | ExponentPushToken |
| 지오코딩 | 카카오 Local API | 주소 → 위경도 |
| 날씨 | 기상청 단기·초단기예보 | [WEATHER-API.md](./WEATHER-API.md) |

## 모노레포 구조

```
grab-umbrella/
├── server/                  # Go 백엔드
│   ├── cmd/
│   │   ├── api/             # 웹 서비스 (POST /sync, GET /forecast, /healthz)
│   │   ├── cron/            # 출발 30분 전 푸시 발송 (별도 서비스)
│   │   ├── migrate/         # DB 마이그레이션 적용 (배포 대상)
│   │   ├── peek/            # [개발] 기기 등록 상태 조회 (psql 부재 대응)
│   │   ├── diag/            # [개발] 격자·시각별 예보 슬롯 점검
│   │   └── testpush/        # [개발] 최신 정식 토큰으로 실발송 테스트
│   ├── internal/
│   │   ├── handler/         # HTTP 핸들러 (/sync, /forecast)
│   │   ├── store/           # DB 접근 (devices, DueDevices 매칭)
│   │   ├── weather/         # 기상청 클라이언트 + 캐싱 + 재시도 + 우산 판정
│   │   ├── grid/            # 위경도 → LCC DFS 격자(nx,ny)
│   │   ├── geocode/         # 카카오 주소 → 위경도
│   │   └── push/            # Expo Push 발송
│   └── migrations/          # 001_init.sql, 002_add_address.sql
├── app/                     # React Native (Expo) 앱
│   └── src/
│       ├── screens/         # MainScreen, SettingsScreen
│       ├── components/      # CommuteCard, HourlySheet, AddressSearch, PermissionPrimer
│       ├── lib/             # api.ts, push.ts, types.ts, format.ts
│       └── storage/         # settings.ts (AsyncStorage, 설정의 주인)
└── docs/                    # 이 문서들
    ├── PROJECT.md           # ← 지금 이 문서 (진입점)
    ├── ARCHITECTURE.md      # 서버/앱 실제 구현 상세
    ├── WEATHER-API.md       # 기상청 API·base_time·우산 판정
    ├── DECISIONS.md         # 확정된 설계 결정 + 미구현 TODO
    ├── design-main-screen.md# 메인 화면 디자인 확정안
    └── kma-guide/           # 기상청 공식 가이드(바이너리는 gitignore)
```

## 빠른 시작

### 서버
```bash
cd server
# .env 에 DATABASE_URL, KMA_SERVICE_KEY, KAKAO_REST_API_KEY 채우기 (gitignored)
go run ./cmd/migrate   # 최초 1회 — 테이블 생성
go run ./cmd/api       # 웹 서비스 (:8080)
```

### 앱
```bash
cd app
pnpm install           # pnpm 사용 (npm 아님), node-linker=hoisted
pnpm expo start --lan  # 실기기는 app.json extra.apiBaseUrl 의 LAN IP 로 서버 접근
```

> 실기기 테스트: Expo Go(SDK 54) 필요. `app.json`의 `extra.apiBaseUrl`을 Mac LAN IP로 맞춰야 실기기가 로컬 서버에 닿는다.

## 현재 상태 (2026-06 기준)

구현 순서(spec §9) 1~6단계 완료. 실기기 푸시 파이프라인 전 구간 검증됨(토큰→Expo→APNs→단말, cron 매칭→발송→중복방지).

자세한 완성도·미구현 항목은 [DECISIONS.md](./DECISIONS.md#구현-완성도).
