# 기상청 API & 우산 판정 로직

> 이 프로젝트에서 가장 까다로웠던 부분. base_time 마진과 우산 판정 규칙을 코드 기준으로 정리.
> 공식 가이드 출처는 [kma-guide/README.md](./kma-guide/README.md).

## 엔드포인트

베이스: `http://apis.data.go.kr/1360000/VilageFcstInfoService_2.0` (활용신청 1회로 전체 기능 자동승인)

| 용도 | 기능 | 발표 주기 | 예보 범위 | 기온 필드 |
|---|---|---|---|---|
| 메인 화면 (멀리) | `getVilageFcst` (단기예보) | 1일 8회(3시간) | 글피까지, 1시간 단위 | TMP |
| 푸시 (임박) | `getUltraSrtFcst` (초단기예보) | 매시각 | +6시간, 1시간 단위 | T1H |
| "지금 바깥"(선택) | `getUltraSrtNcst` (초단기실황) | 매시각 | 현재 관측 | (미사용) |

**원칙: 멀리 보는 건 단기예보, 임박한 건 초단기예보.** 6시간 이내 시점은 초단기, 그 외 단기. 초단기 실패 시 단기로 graceful 폴백.

요청 파라미터: `serviceKey, pageNo, numOfRows, dataType=JSON, base_date(YYYYMMDD), base_time(HHmm), nx, ny`.
응답: `fcstDate / fcstTime / category / fcstValue` 항목 배열.

## ⚠️ base_time 안전 마진 — v1이 막혔던 핵심

**문제:** 정각에 발표돼도 데이터가 실제 채워지는 건 발표 후 얼마간 지나서다. 정각 직후 호출하면 빈 응답. v1은 이걸로 막혔다.

**해법:** base_time을 "정각"이 아니라 "안전 마진 둔 직전 발표본"으로 잡는다. `internal/weather/basetime.go`.

### 단기예보 (getVilageFcst)

- 발표시각: **02, 05, 08, 11, 14, 17, 20, 23시**
- **마진: 15분** (`vilageSafetyMargin`)
  - 공식 가이드 명시값은 **발표 후 10분**(02:10, 05:10 …). 생성/네트워크 여유를 더해 **15분**으로 둠.
  - 예) 08:20 호출 → 08시 발표본(08:15 지남). 08:10 호출 → 05시 발표본(08:15 아직).
  - 자정 경계: 02시 전이면 전날 23시 발표본.
- ⚠️ **흔한 오해: 45분이 아니다.** 45분은 *초단기예보*의 마진. 단기예보에 45분은 과도.

### 초단기예보 (getUltraSrtFcst)

- 발표시각: **매시 30분**
- **마진: 45분** (`ultraSrtSafetyMargin`)
  - now 분 < 45 → 한 시간 전 30분 발표본. now 분 ≥ 45 → 이번 시각 30분 발표본.
  - 예) 16:20 → 15:30 발표본. 16:50 → 16:30 발표본.
- 예보 범위: 발표 기준 +6시간.

## 캐싱 & 재시도 (`weather.go`, `cache.go`)

- **캐시 단위:** 격자(nx,ny) + 발표본(baseDate,baseTime) + 종류(vilage/ultra). 같은 동네 사용자 여럿이면 1회 호출로 공유. 발표본 바뀌면 자동 무효화, 같은 격자는 최신 발표본 1개만 유지.
- **재시도:** 최대 3회, 지수 backoff(0.5s→1s). **429 또는 5xx만** 재시도, 그 외 4xx는 즉시 실패.
- **폴백:** 초단기 실패 시 단기예보로 (spec §9-2 graceful).

## category 코드표 (`mapping.go`)

| 코드 | 의미 | 값 |
|---|---|---|
| **SKY** | 하늘상태 | 1=맑음, 3=구름많음, 4=흐림 |
| **PTY** | 강수형태 | 0=없음, 1=비, 2=비/눈, 3=눈, 4=소나기 |
| **POP** | 강수확률 | 0~100(%) |
| **TMP** / **T1H** | 기온 | 단기 / 초단기. `tempC`가 TMP 우선, 없으면 T1H 폴백 |
| **PCP** / **SNO** | 강수량/적설 | 비수치 문자열("강수없음" 등). `PrecipText`로 정규화(현재 카드엔 미노출) |

## 우산 판정 — 핵심 규칙

### 단일 시점 판정 (`NeedUmbrella`)

```go
const popUmbrellaThreshold = 60
func NeedUmbrella(pty, pop int) bool {
    return pty != 0 || pop >= popUmbrellaThreshold  // 강수형태 있음 OR 강수확률 60%+
}
```

### 윈도우 판정 (`WindowNeedUmbrella` / `windowScan`) — 중요

우산 판정은 **anchor 정시 1점이 아니라 출퇴근 윈도우 전체** 기준이다.

> **왜:** 퇴근 19시 정시는 맑아도 19~20시에 소나기가 있으면 우산이 필요하다. anchor 1점만 보면 이걸 놓친다. 윈도우 안 **어느 한 시점이라도** 우산 필요면 true.

**윈도우 범위** (`slice.go`, 비대칭):
```go
morningHoursBefore = 2   // 출근: 가는 길이 중요 → 이전을 더 본다
morningHoursAfter  = 1   //        (2시간 전 ~ 1시간 후)
eveningHoursBefore = 1   // 퇴근: 이후가 중요 → 이후를 더 본다
eveningHoursAfter  = 2   //        (1시간 전 ~ 2시간 후)
```

- `/forecast`(앱)와 cron이 **동일 기준**으로 판정(`WindowNeedUmbrella` 공유).
- `windowScan`은 우산 필요한 **첫 시점의 시각·강수형태**도 반환 → `UmbrellaReason` 생성.

### UmbrellaReason — 혼란 방지 부제목

대표값(anchor)이 맑은데 윈도우 때문에 우산이 필요할 때, "왜 챙겨야 하는지"를 설명하는 근거 문구.

- `windowScan`이 첫 우산 시점을 찾아 `umbrellaReasonText(hour, pty)` → 예 **"19시부터 소나기"**.
- **anchor 자체가 비면 빈 문자열.** 카드 대표값이 이미 비라 근거가 자명 → 중복/혼란 방지.
- 앱은 카드 부제목 + 섹션 결론 아래에 표시. anchor가 비라 reason이 비면 `ptyText`로 폴백("비"/"소나기").

## 회귀 테스트 (`slice_test.go`)

윈도우 판정/근거는 회귀로 고정돼 있다:
- `TestWindowNeedUmbrella` — 윈도우 내 소나기→필요 / 전부 맑음→불필요 / 윈도우 밖 소나기→무시
- `TestBuildSlotCardUmbrellaReason` — anchor 맑고 이후 소나기→reason 설정 / anchor 자체 비→reason 빈값

## 미세먼지 (보류)

에어코리아 API. 위경도→TM변환→근접측정소→측정값 체인이라 무겁고 별도 활용신청 필요. **초기 스코프 제외**(spec §4.7).
