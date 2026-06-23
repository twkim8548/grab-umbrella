# 우산챙겨? (grab-umbrella)

출퇴근길 날씨 알림 앱. 사용자가 실제로 바깥에 있는 두 순간 — **출근 시각, 퇴근 시각** — 의 날씨에만 집중한다.

앱이 답하는 질문은 단 두 개:
- 오늘 출근할 때 우산 챙겨야 해?
- 옷은 어떻게 입어야 해? (어제 대비 체감 포함)

킬러 기능은 화면이 아니라 **푸시**다. 앱을 안 켜도 출발 30분 전에 "우산 챙기세요" 한 줄이 온다.

> 설계 전문: [`weather-fairy-v2-spec.md`](./weather-fairy-v2-spec.md)

## 모노레포 구조

```
grab-umbrella/
├── server/                 # Go 백엔드 (단일 바이너리)
│   ├── cmd/
│   │   ├── api/            # 웹 서비스 (POST /sync, GET /forecast, GET /forecast/now)
│   │   └── cron/           # Render Cron Job — 출발 30분 전 푸시 발송
│   └── internal/
│       ├── handler/        # HTTP 핸들러
│       ├── store/          # DB 접근 (Postgres: Supabase/Neon)
│       ├── weather/        # 기상청 API 클라이언트 + 캐싱 + 폴백
│       ├── grid/           # 위경도 → LCC DFS 격자(nx,ny) 변환
│       └── push/           # Expo Push 발송
├── app/                    # React Native (Expo) 앱
│   └── src/
│       ├── screens/        # 메인 / 설정
│       ├── components/     # 출근·퇴근 카드 등
│       ├── lib/            # API 클라이언트, 푸시 토큰
│       └── storage/        # AsyncStorage (설정의 주인)
└── docs/                   # 추가 설계·운영 문서
```

## 기술 스택

| 영역 | 선택 |
|---|---|
| 앱 | React Native (Expo) |
| 서버 | Go + Chi (net/http 기반 경량 라우터) |
| 호스팅 | Render (Free) |
| DB | Supabase 또는 Neon (Postgres) — *§10 택1 미확정* |
| 스케줄링 | Render Cron Job (별도 서비스) |
| 푸시 | Expo Push |

**로그인 없음.** 기기 기반 식별 — 식별자는 Expo 푸시 토큰.

## 개발 시작

### 서버
```bash
cd server
cp .env.example .env   # DB URL, 기상청 ServiceKey 등 채우기
go run ./cmd/api
```

### 앱
```bash
cd app
pnpm install
pnpm start      # = expo start
```

## 구현 순서 (spec §9)

1. DB + `/sync` — devices 테이블, 위경도→격자 변환, upsert
2. `/forecast` — 단기예보 조회·가공, 캐싱
3. RN 앱 골격 — 설정 화면, 로컬 저장, /sync 연동
4. 메인 화면 — 출근/퇴근 카드 (Apple HIG)
5. Expo Push 셋업 — 토큰 발급·등록
6. Cron 푸시 — 초단기예보, 중복방지
7. (고도화) 어제 대비 체감, 미세먼지, LLM 메시지
