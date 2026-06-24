# 아키텍처 — 실제 구현 상세

> 코드 기준 구현 맵. 파일:라인은 작성 시점 기준이라 다를 수 있으니 심볼명으로도 찾을 것.
> 큰 그림은 [PROJECT.md](./PROJECT.md), 날씨 로직은 [WEATHER-API.md](./WEATHER-API.md).

## 서버 (Go)

- 모듈: `github.com/twkim8548/grab-umbrella/server`, Go 1.22
- DB 드라이버: `jackc/pgx/v5` (pgxpool), 라우터: `chi/v5`

### API 엔드포인트

| 엔드포인트 | 메서드 | 상태 |
|---|---|---|
| `/healthz` | GET | 헬스체크 |
| `/sync` | POST | 구현 완료 |
| `/forecast` | GET | 구현 완료 |
| `/forecast/now` | GET | **미구현 (501)** — spec §3 선택 항목 |

포트: `PORT` env, 기본 `8080`. 미들웨어: chi Logger, Recoverer.

### POST /sync

앱이 설정 변경 시 호출. `internal/handler/handler.go`.

**입력** (JSON):
```json
{
  "push_token": "ExponentPushToken[...]",
  "home_address": "경기 용인시 수지구 ...",   // 도로명/지번 주소
  "work_address": "서울 강남구 ...",
  "commute_start": "0730",                     // HHmm, KST
  "commute_end": "1900"
}
```

**처리**: 주소 → 카카오 지오코딩(위경도) → `grid.ToGrid`(LCC DFS 격자 nx,ny) → `devices` upsert.
**에러**: 400(필수 누락), 422(주소 못 찾음, 본문에 사유), 500(upsert 실패).

### GET /forecast

메인 화면용. `?push_token=...`로 기기 조회 후 오늘·내일 출퇴근 **4시점**을 반환.

**응답** (JSON):
```json
{
  "today":    { "morning": SlotCard|null, "evening": SlotCard|null },
  "tomorrow": { "morning": SlotCard|null, "evening": SlotCard|null }
}
```
- **이미 지난 시점은 null** (예: 오후엔 `today.morning = null`). 앱이 현재 시각에 맞춰 어느 날을 보여줄지 판단([design-main-screen.md](./design-main-screen.md)).
- 404 → 기기 미등록(신규/미동기화). 앱은 이걸 "에러"가 아니라 "동기화 필요"로 구분.

**SlotCard** 구조 (`internal/weather/slice.go`):
```go
type SlotCard struct {
    SkyText        string        // "맑음"/"구름많음"/"흐림"
    PtyText        string        // "없음"/"비"/"비/눈"/"눈"/"소나기"
    TempC          int           // 기온(℃)
    PopPct         int           // 강수확률(%)
    NeedUmbrella   bool          // 우산 필요 — 출퇴근 윈도우 전체 기준 (WEATHER-API.md)
    UmbrellaReason string        // 대표값(anchor)과 결론이 어긋날 때만 근거 문구. 예 "19시부터 소나기"
    Hourly         []HourlyPoint // 시간별 흐름(점진적 공개용)
}
type HourlyPoint struct { Time string; TempC int; PopPct int; PtyText string }
```

**buildCardForDate** (handler): 슬롯 시각 = fcstDate + `NormalizeToHour(commute)`(정시 내림). 규칙:
1. 그 시각이 now(KST)보다 과거 → `nil,false` (지난 시점은 표시 안 함).
2. 6시간 이내 → 초단기예보 우선. 실패/빈 슬롯이면 단기예보 폴백.
3. 6시간 밖 → 처음부터 단기예보.
4. 단기까지 실패 → `false`로 graceful null.

### Cron 푸시 (`cmd/cron`)

웹과 **별도 서비스**(spin-down 무관). 매 N분 깨어나 흐름:
1. `DueDevices(now, lead)` — "지금부터 lead분 뒤가 출근/퇴근"인 기기만 SELECT.
2. `SlotDateTime` — 슬롯 날짜/시각 결정.
3. `fetchForecast` — 6시간 이내면 초단기, 실패 시 단기 폴백.
4. `WindowNeedUmbrella` — 출퇴근 윈도우 전체로 우산 판정([WEATHER-API.md](./WEATHER-API.md)).
5. 정식 `ExponentPushToken[...]`만 발송(dev 토큰은 skip, MarkPushed 안 함 → 나중에 정식 토큰 붙으면 수신).
6. `MarkPushed` — `last_morning/evening_push_date`에 오늘 날짜 기록(중복 방지).

**환경변수**:
| 변수 | 기본 | 용도 |
|---|---|---|
| `PUSH_LEAD_MINUTES` | 30 | 출발 몇 분 전 발송 |
| `DATABASE_URL` | (필수) | DB 연결 |
| `KMA_SERVICE_KEY` | (필수) | 기상청 키 |
| `CRON_NOW` | (없음) | **테스트용** 기준 시각 주입 "2006-01-02 15:04" |
| `CRON_FORCE_SEND` | (없음) | **테스트용** "1"이면 비 여부 무관 강제 발송 |

> ⚠️ `CRON_NOW`는 시각만 속이고 **기상청은 진짜 현재 기준 데이터를 준다.** 과거 시각 슬롯은 "no forecast"로 skip된다. 발송까지 검증하려면 실재하는 미래 슬롯에 맞춰야 함. 운영에선 두 변수 모두 미설정.

**발송 문구** (`buildMessage`):
```
제목: "우산 챙기세요! ☔️"
본문(출근): "오늘 출근길에 비소식이 있어요"
본문(퇴근): "오늘 퇴근길에 비소식이 있어요"
```
비가 안 오면 발송 안 함(`shouldSend=false`). 체감(옷차림) 기반 발송은 미구현(§9-7 TODO).

### DB 스키마 (`migrations/`)

```sql
-- 001_init.sql
CREATE TABLE devices (
    push_token             TEXT PRIMARY KEY,   -- Expo 토큰 (익명 식별자)
    home_nx, home_ny       INT NOT NULL,       -- 집 격자 (LCC DFS)
    work_nx, work_ny       INT NOT NULL,       -- 회사 격자
    commute_start          TEXT NOT NULL,      -- "0730" HHmm KST
    commute_end            TEXT NOT NULL,      -- "1900"
    last_morning_push_date TEXT,               -- "YYYYMMDD" 중복방지
    last_evening_push_date TEXT,
    last_synced_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_devices_commute_start ON devices (commute_start);
CREATE INDEX idx_devices_commute_end   ON devices (commute_end);

-- 002_add_address.sql — 표시용 보조 주소 (격자가 실제 위치 식별자)
ALTER TABLE devices ADD COLUMN home_address TEXT;
ALTER TABLE devices ADD COLUMN work_address TEXT;
```

마이그레이션은 `psql` 없이 `cmd/migrate`(pgx)로 적용.

### DueDevices 매칭 (`internal/store/store.go`)

- `tickInterval = 10분`. `now+lead`의 타겟 시각이 commute 시각의 `[target, target+10분)` 구간에 들면 그 슬롯(morning/eveing)이 due.
- `dueSlot`은 순수 함수(DB 불필요, 테스트 가능). `SlotMorning="morning"`, `SlotEvening="evening"`.
- morning이면 집 격자(home_nx/ny), evening이면 회사 격자(work_nx/ny)로 예보 조회.

### 개발 도구 (cmd/, 운영 배포 아님)

| 도구 | 용도 |
|---|---|
| `peek` | devices 등록/토큰/푸시기록 조회 (psql 부재 환경 대응) |
| `diag <nx> <ny> <HHmm> [YYYYMMDD]` | 격자·시각별 단기예보 슬롯·윈도우 판정·시간별 흐름 출력 |
| `testpush ["ExponentPushToken[...]"]` | DB 최신 정식 토큰(또는 인자)으로 실제 푸시 1발 |

## 앱 (React Native + Expo SDK 54)

### 진입 / 네비게이션 (`App.tsx`)

- `@react-navigation/native-stack`. 두 화면 `Main` / `Settings`. Settings를 push해서 **iOS edge swipe 뒤로가기** + 네이티브 전환 기본 제공. 각 화면이 자체 헤더를 그리므로 `headerShown: false`.
- 모듈 로드 시 `Notifications.setNotificationHandler`로 **포그라운드 알림 배너** 표시(SDK 53+ 기본은 숨김. `shouldShowBanner`/`shouldShowList` 사용 — 구 `shouldShowAlert` 대체).
- **Priming 게이트**: 앱 첫 실행 시 권한이 `undetermined`면 `PermissionPrimer`(앱 자체 안내) 먼저 → [허용] 누른 시점에 시스템 권한 요청. `PRIMED_KEY`로 1회만.

### MainScreen (`src/screens/MainScreen.tsx`)

- **날짜 섹션 패러다임**: `pickSections`가 데이터로 섹션 결정 — 출근 전=오늘만, 출퇴근 사이=오늘+내일, 퇴근 후=내일만. 자세히는 [design-main-screen.md](./design-main-screen.md).
- 각 섹션: 날짜 헤더 + 우산 결론(그 날 살아있는 슬롯 중 하나라도 needUmbrella) + 카드들. 결론 아래 **이유**(`buildSectionReason`: "퇴근길 19시부터 소나기")는 우산 필요할 때만.
- 로드 상태: `loading / no-settings / sync-needed(404) / error / ready`.
- **당겨서 새로고침**: `RefreshControl`. `load(silent=true)`로 화면 깜빡임 없이 데이터만 갱신.

### CommuteCard (`src/components/CommuteCard.tsx`)

- 메인 아이콘 = **그 시각 실제 날씨**(`weatherEmoji(ptyText, skyText)`), 우산 여부 아님. "18시 맑은데 ☔️" 모순 방지.
  - 비🌧 비/눈🌨 눈❄️ 소나기🌦 / 맑음☀️ 구름많음⛅️ 흐림☁️
- 우산 필요 + `umbrellaReason` 있으면 카드에도 "☔️ 19시부터 소나기" 부제목.
- `past` 카드는 흐리게 "지났어요". 탭하면 `HourlySheet`(시간별 흐름).

### 기타 컴포넌트

| 컴포넌트 | 역할 |
|---|---|
| `HourlySheet` | 시간별 흐름 하단 시트 (Animated fade+slide, 열기/닫기 양방향) |
| `AddressSearch` | Daum 우편번호 위젯을 `react-native-webview`로 직접 구현(@actbase 라이브러리 사파리 탈출 버그로 교체) |
| `PermissionPrimer` | HIG 권한 프라이밍 ([허용]/[나중에]) |

### lib

- `api.ts` — `BASE_URL = Constants.expoConfig?.extra?.apiBaseUrl ?? "http://192.168.0.79:8080"`. `sync`, `getForecast`(404→`NOT_REGISTERED`).
- `push.ts` — `getPushToken`: EAS projectId + 권한 granted면 정식 `ExponentPushToken`, 아니면 `dev-<deviceId>` 폴백(항상 토큰 보장, forecast 호출용). `ensureNotificationPermission`, `getNotificationPermissionStatus` 분리.
- `types.ts` — `Settings`(로컬 주인), `SlotForecast`, `DayForecast`, `ForecastResponse`. `umbrellaReason: string` 포함.

### storage

- `settings.ts` — AsyncStorage 키 `grab-umbrella:settings`, JSON. **설정의 주인**(DB는 사본).
