// Package weather 는 기상청 단기예보 조회서비스 클라이언트다. spec §4.
//
// 원칙: 멀리 보는 건 단기예보(getVilageFcst, 메인),
//       임박한 건 초단기예보(getUltraSrtFcst, 푸시).
package weather

import "context"

// Client 는 기상청 API 호출 + 캐싱 + 폴백을 담당한다.
type Client struct {
	serviceKey string
	baseURL    string
	// TODO: 캐시(같은 격자·같은 발표본 1회만 조회, spec §4.6),
	//       httpClient(타임아웃·재시도), 폴백(초단기 실패 시 단기예보).
}

func New(serviceKey, baseURL string) *Client {
	return &Client{serviceKey: serviceKey, baseURL: baseURL}
}

// SlotForecast 는 한 시점(출근 또는 퇴근)의 가공된 예보다.
type SlotForecast struct {
	SkyText   string // 맑음/구름많음/흐림 (SKY)
	PtyText   string // 없음/비/비눈/눈/소나기 (PTY)
	TempC     int    // 기온 (T1H 또는 TMP)
	PopPct    int    // 강수확률 (POP) — spec §4.4: v1에 없던 핵심 추가
	NeedUmbrella bool
	// TODO: 어제 대비 체감, 시간별 흐름(점진적 공개용 슬라이스) 등
}

// VilageForecast (단기예보) — 메인 화면용. spec §3 GET /forecast.
// base_time 안전 마진은 spec §4.5 참고 (정각 아닌 직전 발표본 사용).
func (c *Client) VilageForecast(ctx context.Context, nx, ny int, hhmm string) (*SlotForecast, error) {
	// TODO: getVilageFcst 호출 → POP/PTY/SKY/WSD/TMN/TMX 파싱 → SlotForecast
	return nil, errNotImplemented
}

// UltraSrtForecast (초단기예보) — 푸시용. +6시간, 1시간 단위.
func (c *Client) UltraSrtForecast(ctx context.Context, nx, ny int, hhmm string) (*SlotForecast, error) {
	// TODO: getUltraSrtFcst 호출 → T1H/PTY/SKY/RN1 파싱
	return nil, errNotImplemented
}

// BaseTime 은 안전 마진을 둔 직전 발표본의 base_date/base_time 을 계산한다. spec §4.5.
// TODO: v1 stores/weather.ts 로직 포팅 + 45분 마진 보강.
func BaseTime(/* now time.Time, kind FcstKind */) (baseDate, baseTime string) {
	return "", ""
}

type sentinelError string

func (e sentinelError) Error() string { return string(e) }

const errNotImplemented = sentinelError("weather: not implemented yet")
