package handler

import (
	"net/http"
	"strings"

	"github.com/twkim8548/grab-umbrella/server/internal/weather"
)

// mock.go 는 개발용 시나리오 프리뷰다. /forecast?mock=... 으로 가짜 4시점 날씨를
// 내려 앱 UI 를 실제 데이터 없이 시나리오별로 확인한다. 운영 영향 없음(파라미터 없으면 무시).
//
// 사용:
//   /forecast?mock=sunny                       모든 시점 동일 시나리오
//   /forecast?mock=rain,sunny,shower-later,cloudy   오늘출근,오늘퇴근,내일출근,내일퇴근 순
//   값 일부 생략 가능(빈 칸은 none): mock=,rain → 오늘출근 none, 오늘퇴근 rain
//
// 시나리오 어휘:
//   sunny         맑음, 우산 X
//   cloudy        구름많음, 우산 X
//   rain          비(anchor 부터), 우산 O
//   shower        소나기(anchor 부터), 우산 O
//   shower-later  anchor 는 맑고 윈도우 뒤에 소나기 → reason "N시부터 소나기"
//   none / (빈값) 데이터 없음(지난 시점) → null

// mockScenario 는 한 시점의 가짜 카드를 만든다. anchorHour 는 reason 문구 계산용(예 19).
// ok=false 면 그 시점은 null(none/빈값).
func mockScenario(name string, anchorHour int) (weather.SlotCard, bool) {
	name = strings.TrimSpace(name)
	switch name {
	case "", "none":
		return weather.SlotCard{}, false

	case "sunny":
		return weather.SlotCard{
			SkyText: "맑음", PtyText: "없음", TempC: 23, PopPct: 0,
			NeedUmbrella: false, Hourly: mockHourly(anchorHour, "맑음", 0),
		}, true

	case "cloudy":
		return weather.SlotCard{
			SkyText: "구름많음", PtyText: "없음", TempC: 21, PopPct: 20,
			NeedUmbrella: false, Hourly: mockHourly(anchorHour, "없음", 20),
		}, true

	case "rain":
		return weather.SlotCard{
			SkyText: "흐림", PtyText: "비", TempC: 18, PopPct: 80,
			NeedUmbrella: true, Hourly: mockHourly(anchorHour, "비", 80),
		}, true

	case "shower":
		return weather.SlotCard{
			SkyText: "구름많음", PtyText: "소나기", TempC: 24, PopPct: 60,
			NeedUmbrella: true, Hourly: mockHourly(anchorHour, "소나기", 60),
		}, true

	case "shower-later":
		// anchor(정시)는 맑지만 +1h 부터 소나기 → 윈도우 판정 true, reason "N+1시부터 소나기".
		h := mockHourly(anchorHour, "없음", 20)
		if len(h) >= 3 {
			h[len(h)-2].PtyText = "소나기"
			h[len(h)-2].PopPct = 60
			h[len(h)-1].PtyText = "소나기"
			h[len(h)-1].PopPct = 60
		}
		reason := umbrellaReasonAt(anchorHour + 1)
		return weather.SlotCard{
			SkyText: "구름많음", PtyText: "없음", TempC: 22, PopPct: 20,
			NeedUmbrella: true, UmbrellaReason: reason, Hourly: h,
		}, true

	default:
		// 알 수 없는 값 → 맑음으로(견고성).
		return weather.SlotCard{
			SkyText: "맑음", PtyText: "없음", TempC: 23, PopPct: 0,
			NeedUmbrella: false, Hourly: mockHourly(anchorHour, "맑음", 0),
		}, true
	}
}

// mockHourly 는 anchor 전후 4시점(anchor-1 ~ anchor+2)을 만든다. 표시 검증용.
func mockHourly(anchorHour int, pty string, pop int) []weather.HourlyPoint {
	out := make([]weather.HourlyPoint, 0, 4)
	for off := -1; off <= 2; off++ {
		hh := ((anchorHour+off)%24 + 24) % 24
		out = append(out, weather.HourlyPoint{
			Time:    twoDigit(hh) + "00",
			TempC:   22,
			PopPct:  pop,
			PtyText: pty,
		})
	}
	return out
}

func umbrellaReasonAt(hour int) string {
	hour = ((hour % 24) + 24) % 24
	return itoa(hour) + "시부터 소나기"
}

func twoDigit(n int) string {
	if n < 10 {
		return "0" + itoa(n)
	}
	return itoa(n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// writeMockForecast 는 mock 파라미터를 파싱해 가짜 4시점 응답을 쓴다.
// 출근 anchor 는 9시, 퇴근 anchor 는 19시로 고정(시나리오 프리뷰라 reason 시각만 그럴듯하게).
func writeMockForecast(w http.ResponseWriter, mock string) {
	parts := strings.Split(mock, ",")
	get := func(i int) string {
		if i < len(parts) {
			return parts[i]
		}
		// 값이 하나뿐이면 모든 시점에 같은 시나리오 적용.
		if len(parts) == 1 {
			return parts[0]
		}
		return ""
	}

	const morningAnchor, eveningAnchor = 9, 19
	var resp forecastResponse
	if c, ok := mockScenario(get(0), morningAnchor); ok {
		resp.Today.Morning = &c
	}
	if c, ok := mockScenario(get(1), eveningAnchor); ok {
		resp.Today.Evening = &c
	}
	if c, ok := mockScenario(get(2), morningAnchor); ok {
		resp.Tomorrow.Morning = &c
	}
	if c, ok := mockScenario(get(3), eveningAnchor); ok {
		resp.Tomorrow.Evening = &c
	}
	writeJSON(w, resp)
}
