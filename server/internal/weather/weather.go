// Package weather 는 기상청 단기예보 조회서비스 클라이언트다. spec §4.
//
// 원칙: 멀리 보는 건 단기예보(getVilageFcst, 메인),
//
//	임박한 건 초단기예보(getUltraSrtFcst, 푸시).
package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Client 는 기상청 API 호출을 담당한다.
type Client struct {
	serviceKey string
	baseURL    string
	httpClient *http.Client
	cache      *forecastCache // 같은 격자·같은 발표본 1회만 조회(spec §4.6).
}

func New(serviceKey, baseURL string) *Client {
	return &Client{
		serviceKey: serviceKey,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		cache:      newForecastCache(),
	}
}

// vilageNumOfRows 는 한 번에 받을 항목 수다. 단기예보는 글피까지 시간대×카테고리라
// 항목이 많으므로 충분히 크게 잡아 페이지네이션 없이 전부 받는다.
const vilageNumOfRows = 1000

// ultraSrtNumOfRows 는 초단기예보 한 번에 받을 항목 수다. 초단기예보는 +6시간(시간당
// ~11개 카테고리)이라 항목이 적으므로 단기예보보다 작게 잡아도 전부 받는다.
const ultraSrtNumOfRows = 300

// FcstItem 은 기상청 응답의 개별 예보 항목(1행)이다.
type FcstItem struct {
	BaseDate  string `json:"baseDate"`
	BaseTime  string `json:"baseTime"`
	Category  string `json:"category"`
	FcstDate  string `json:"fcstDate"`
	FcstTime  string `json:"fcstTime"`
	FcstValue string `json:"fcstValue"`
	Nx        int    `json:"nx"`
	Ny        int    `json:"ny"`
}

// kmaResponse 는 getVilageFcst 응답 구조다. response.body.items.item[].
type kmaResponse struct {
	Response struct {
		Header struct {
			ResultCode string `json:"resultCode"`
			ResultMsg  string `json:"resultMsg"`
		} `json:"header"`
		Body struct {
			Items struct {
				Item []FcstItem `json:"item"`
			} `json:"items"`
		} `json:"body"`
	} `json:"response"`
}

// VilageForecast (단기예보) — getVilageFcst 를 호출해 해당 격자의 모든 예보 항목을
// 파싱해 반환한다. base_date/base_time 은 BaseTime 으로 안전 마진을 두고 산출한다.
// spec §4. 가공/슬라이스/캐싱은 다음 단계.
func (c *Client) VilageForecast(ctx context.Context, nx, ny int) ([]FcstItem, error) {
	baseDate, baseTime := BaseTime(time.Now())

	// 캐시 적중: 같은 격자·같은 발표본은 재호출 없이 공유한다(spec §4.6).
	if c.cache != nil {
		if items, ok := c.cache.get(kindVilage, nx, ny, baseDate, baseTime); ok {
			return items, nil
		}
	}

	q := url.Values{}
	q.Set("serviceKey", c.serviceKey)
	q.Set("dataType", "JSON")
	q.Set("numOfRows", strconv.Itoa(vilageNumOfRows))
	q.Set("pageNo", "1")
	q.Set("base_date", baseDate)
	q.Set("base_time", baseTime)
	q.Set("nx", strconv.Itoa(nx))
	q.Set("ny", strconv.Itoa(ny))

	endpoint := c.baseURL + "/getVilageFcst?" + q.Encode()

	body, err := c.getWithRetry(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	items, err := parseVilageFcst(body)
	if err != nil {
		return nil, err
	}
	if c.cache != nil {
		c.cache.put(kindVilage, nx, ny, baseDate, baseTime, items)
	}
	return items, nil
}

// getWithRetry 는 endpoint 를 GET 하고, 429/5xx(일시적 실패)면 지수 backoff 로 재시도한다.
// 기상청은 느리고 실패가 잦다(spec §4.6) — 특히 429(rate limit)·5xx 는 잠깐 뒤 보통 풀린다.
// 4xx(429 제외)는 재시도해도 의미 없으므로 즉시 실패한다.
func (c *Client) getWithRetry(ctx context.Context, endpoint string) ([]byte, error) {
	const maxAttempts = 3
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			// 지수 backoff: 0.5s, 1s. context 취소되면 즉시 중단.
			backoff := time.Duration(250<<attempt) * time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("weather: build request: %w", err)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("weather: http do: %w", err)
			continue // 네트워크 오류는 재시도.
		}

		if resp.StatusCode == http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				return nil, fmt.Errorf("weather: read body: %w", err)
			}
			return body, nil
		}

		status := resp.StatusCode
		resp.Body.Close()
		lastErr = fmt.Errorf("weather: http status %d", status)

		// 429(rate limit) 와 5xx 만 재시도. 그 외 4xx 는 즉시 실패.
		if status != http.StatusTooManyRequests && status < 500 {
			return nil, lastErr
		}
	}
	return nil, fmt.Errorf("weather: retries exhausted: %w", lastErr)
}

// parseVilageFcst 는 getVilageFcst 응답 본문을 파싱한다. resultCode 가 "00"이 아니면
// 에러를 반환한다(헤더의 resultCode/resultMsg 사용).
func parseVilageFcst(body []byte) ([]FcstItem, error) {
	var r kmaResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("weather: parse json: %w", err)
	}
	if r.Response.Header.ResultCode != "00" {
		return nil, fmt.Errorf("weather: api error %s: %s",
			r.Response.Header.ResultCode, r.Response.Header.ResultMsg)
	}
	return r.Response.Body.Items.Item, nil
}

// SlotForecast 는 한 시점(출근 또는 퇴근)의 가공된 예보다.
type SlotForecast struct {
	SkyText      string // 맑음/구름많음/흐림 (SKY)
	PtyText      string // 없음/비/비눈/눈/소나기 (PTY)
	TempC        int    // 기온 (단기예보는 TMP)
	PopPct       int    // 강수확률 (POP) — spec §4.4
	NeedUmbrella bool
	// TODO(다음 단계): 어제 대비 체감, 시간별 흐름(점진적 공개용 슬라이스) 등.
}

// SlotForecast 는 파싱된 항목들에서 특정 시각(fcstDate="YYYYMMDD", fcstTime="HHmm")의
// 예보를 골라 SlotForecast 로 정리한다. 단기/초단기 items 를 모두 처리하기 위해 기온은
// TMP(단기) 우선, 없으면 T1H(초단기)로 폴백한다. POP/PTY/SKY 코드명은 양쪽 동일하다.
func SlotForecastAt(items []FcstItem, fcstDate, fcstTime string) (SlotForecast, bool) {
	byCategory := map[string]string{}
	found := false
	for _, it := range items {
		if it.FcstDate == fcstDate && it.FcstTime == fcstTime {
			byCategory[it.Category] = it.FcstValue
			found = true
		}
	}
	if !found {
		return SlotForecast{}, false
	}

	sky := atoiDefault(byCategory["SKY"], 0)
	pty := atoiDefault(byCategory["PTY"], 0)
	pop := atoiDefault(byCategory["POP"], 0)
	temp := tempC(byCategory)

	return SlotForecast{
		SkyText:      skyText(sky),
		PtyText:      ptyText(pty),
		TempC:        temp,
		PopPct:       pop,
		NeedUmbrella: NeedUmbrella(pty, pop),
	}, true
}

// tempC 는 한 시각의 카테고리 맵에서 기온(°C)을 뽑는다. 단기예보는 TMP, 초단기예보는 T1H
// 코드를 쓰므로, TMP 우선·없으면 T1H 폴백으로 양쪽 items 를 모두 처리한다. 둘 다 없으면 0.
func tempC(byCategory map[string]string) int {
	if v, ok := byCategory["TMP"]; ok {
		return atoiDefault(v, 0)
	}
	return atoiDefault(byCategory["T1H"], 0)
}

// atoiDefault 는 문자열을 정수로 변환하되 실패하면 def 를 반환한다.
// 기상청 일부 값은 "강수없음" 등 비수치 문자열일 수 있으므로 안전하게 처리한다.
func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

// UltraSrtForecast (초단기예보) — getUltraSrtFcst 를 호출해 해당 격자의 예보 항목을 파싱해
// 반환한다. 예보 범위는 발표시점부터 +6시간. base_date/base_time 은 UltraSrtBaseTime 으로
// 안전 마진(매시 45분 제공)을 두고 산출한다. 응답 구조는 단기예보와 동일하므로
// parseVilageFcst 를 재사용한다. 캐시는 단기예보와 종류(kind)로 구분한다(spec §4.6).
func (c *Client) UltraSrtForecast(ctx context.Context, nx, ny int) ([]FcstItem, error) {
	baseDate, baseTime := UltraSrtBaseTime(time.Now())

	if c.cache != nil {
		if items, ok := c.cache.get(kindUltra, nx, ny, baseDate, baseTime); ok {
			return items, nil
		}
	}

	q := url.Values{}
	q.Set("serviceKey", c.serviceKey)
	q.Set("dataType", "JSON")
	q.Set("numOfRows", strconv.Itoa(ultraSrtNumOfRows))
	q.Set("pageNo", "1")
	q.Set("base_date", baseDate)
	q.Set("base_time", baseTime)
	q.Set("nx", strconv.Itoa(nx))
	q.Set("ny", strconv.Itoa(ny))

	endpoint := c.baseURL + "/getUltraSrtFcst?" + q.Encode()

	body, err := c.getWithRetry(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	items, err := parseVilageFcst(body)
	if err != nil {
		return nil, err
	}
	if c.cache != nil {
		c.cache.put(kindUltra, nx, ny, baseDate, baseTime, items)
	}
	return items, nil
}
