package weather

import (
	"fmt"
	"time"
)

// 시간별 흐름 윈도우 (spec §3·§7.1). 명세상 "와이어프레임 시 확정"이라 미정값이므로
// 상수로 분리해 나중에 조정 가능하게 둔다. 단기예보는 1시간 단위이므로 시간 단위로 잡는다.
//
// 출근: 가는 길이 중요 → 이전 쪽을 더 본다(2시간 전 ~ 1시간 후).
// 퇴근: 이후가 중요 → 이후 쪽을 더 본다(1시간 전 ~ 2시간 후).
//
// TODO(와이어프레임 확정): 아래 범위는 잠정값. 디자인 확정 시 조정.
const (
	morningHoursBefore = 2 // 출근 시각 기준 이전 시간 수
	morningHoursAfter  = 1 // 출근 시각 기준 이후 시간 수
	eveningHoursBefore = 1 // 퇴근 시각 기준 이전 시간 수
	eveningHoursAfter  = 2 // 퇴근 시각 기준 이후 시간 수
)

// HourlyPoint 는 시간별 흐름의 한 점이다. 앱 ForecastResponse.hourly[] 와 일치.
type HourlyPoint struct {
	Time    string `json:"time"`    // "0800"
	TempC   int    `json:"tempC"`   // TMP
	PopPct  int    `json:"popPct"`  // POP
	PtyText string `json:"ptyText"` // 없음/비/눈/소나기
}

// SlotCard 는 GET /forecast 응답의 카드 한 장(출근 또는 퇴근)이다. 앱 SlotForecast 와 일치.
// 필드명은 JSON 태그로 camelCase 고정.
type SlotCard struct {
	SkyText      string        `json:"skyText"`
	PtyText      string        `json:"ptyText"`
	TempC        int           `json:"tempC"`
	PopPct       int           `json:"popPct"`
	NeedUmbrella bool          `json:"needUmbrella"`
	// UmbrellaReason 은 카드 대표값(anchor)과 우산 결론이 어긋날 때의 근거 문구다.
	// 예: anchor 18시는 맑지만 윈도우 안 19시부터 소나기 → "19시부터 소나기".
	// anchor 자체가 이미 비라 대표값과 결론이 일치하면 빈 문자열(부제목 불필요).
	UmbrellaReason string        `json:"umbrellaReason"`
	Hourly         []HourlyPoint `json:"hourly"`
}

// BuildSlotCard 는 파싱된 items 에서 한 슬롯 카드(예보 + 시간별 흐름)를 조립한다.
// 핸들러를 얇게 유지하기 위한 순수 함수다. anchorTime 은 정시("HH00")여야 한다.
// 해당 시각 데이터가 없으면(SlotForecastAt ok=false) ok=false 를 반환해 호출부가
// 그 카드를 null 로 graceful 하게 내릴 수 있게 한다(spec §9-2 폴백).
func BuildSlotCard(items []FcstItem, anchorDate, anchorTime string, before, after int) (SlotCard, bool) {
	slot, ok := SlotForecastAt(items, anchorDate, anchorTime)
	if !ok {
		return SlotCard{}, false
	}
	hourly := HourlySlice(items, anchorDate, anchorTime, before, after)
	if hourly == nil {
		hourly = []HourlyPoint{}
	}
	// 우산 판정은 anchor 정시 1점이 아니라 출퇴근 윈도우(before~after) 전체 기준이다.
	// 퇴근 19시는 맑아도 19~20시에 소나기가 있으면 챙겨야 하므로, 윈도우 안 어느
	// 한 시점이라도 우산이 필요하면 true 로 본다(사용자 의도: 하루 단위 우산 판단).
	need, reasonHour, reasonPty := windowScan(items, anchorDate, anchorTime, before, after)

	// 부제목(근거)은 대표값(anchor)과 결론이 어긋날 때만 — 즉 anchor 는 비가 아닌데
	// 윈도우 안 다른 시점 때문에 우산이 필요할 때만 채운다. anchor 가 이미 비면
	// 카드 대표값이 그 자체로 근거이므로 빈 문자열로 둔다(중복/혼란 방지).
	reason := ""
	if need && !slot.NeedUmbrella {
		reason = umbrellaReasonText(reasonHour, reasonPty)
	}

	return SlotCard{
		SkyText:        slot.SkyText,
		PtyText:        slot.PtyText,
		TempC:          slot.TempC,
		PopPct:         slot.PopPct,
		NeedUmbrella:   need,
		UmbrellaReason: reason,
		Hourly:         hourly,
	}, true
}

// windowScan 은 출퇴근 윈도우(anchor 기준 before~after 시간)를 훑어, 우산이 필요한
// 첫 시점을 찾는다. need=우산 필요 여부, hour=첫 시점 시각("HHmm"), pty=그 시점 강수형태.
// 각 시점 판정은 SlotForecastAt 의 NeedUmbrella(PTY/POP)를 재사용한다. anchor 자체도 포함.
func windowScan(items []FcstItem, anchorDate, anchorTime string, before, after int) (need bool, hour, pty string) {
	hh, _ := parseHHmm(anchorTime)
	anchor := time.Date(yearOf(anchorDate), monthOf(anchorDate), dayOf(anchorDate), hh, 0, 0, 0, kst)
	for off := -before; off <= after; off++ {
		t := anchor.Add(time.Duration(off) * time.Hour)
		if slot, ok := SlotForecastAt(items, t.Format("20060102"), t.Format("1504")); ok && slot.NeedUmbrella {
			return true, t.Format("1504"), slot.PtyText
		}
	}
	return false, "", ""
}

// WindowNeedUmbrella 는 윈도우 안 어느 한 시점이라도 우산이 필요하면 true 를 반환한다.
// /forecast(BuildSlotCard)와 cron 이 동일 기준으로 판정하도록 공개한다(windowScan 의 얇은 래퍼).
func WindowNeedUmbrella(items []FcstItem, anchorDate, anchorTime string, before, after int) bool {
	need, _, _ := windowScan(items, anchorDate, anchorTime, before, after)
	return need
}

// umbrellaReasonText 는 "HHmm"+강수형태로 카드 부제목을 만든다. 예: ("1900","소나기") → "19시부터 소나기".
// 분(分)은 정시 슬롯이라 버리고 시(時)만 쓴다. 강수형태가 비면 "비", 소나기면 "소나기" 등 그대로 사용.
func umbrellaReasonText(hour, pty string) string {
	if hour == "" {
		return ""
	}
	hh, _ := parseHHmm(hour)
	if pty == "" || pty == "없음" {
		pty = "비"
	}
	return fmt.Sprintf("%d시부터 %s", hh, pty)
}

// MorningWindow / EveningWindow 는 비대칭 윈도우 상수를 노출한다(핸들러에서 사용).
func MorningWindow() (before, after int) { return morningHoursBefore, morningHoursAfter }
func EveningWindow() (before, after int) { return eveningHoursBefore, eveningHoursAfter }

// NormalizeToHour 는 commute 시각("HHmm", 예 "0830")을 단기예보 정시("HH00")로 내림한다.
// 단기예보는 1시간 단위 정시 슬롯("0800","0900"…)만 존재하므로 분(分)을 버린다.
// 입력이 4자리 HHmm 형식이 아니면 그대로 반환한다(견고성).
func NormalizeToHour(hhmm string) string {
	if len(hhmm) != 4 {
		return hhmm
	}
	for i := 0; i < 4; i++ {
		if hhmm[i] < '0' || hhmm[i] > '9' {
			return hhmm
		}
	}
	return hhmm[:2] + "00"
}

// SlotDateTime 은 슬롯 시각(commute "HHmm")에 해당하는 단기예보 (fcstDate, fcstTime) 을 정한다.
// 규칙(spec §9-2): now(KST) 기준으로 오늘 그 시각이 아직 안 지났으면 오늘, 이미 지났으면 내일.
// fcstTime 은 정시로 내림한 값을 쓴다. now 는 어느 타임존이든 내부에서 KST 로 변환한다.
func SlotDateTime(now time.Time, commute string) (fcstDate, fcstTime string) {
	n := now.In(kst)
	fcstTime = NormalizeToHour(commute)

	hh, mm := parseHHmm(commute)
	slotToday := time.Date(n.Year(), n.Month(), n.Day(), hh, mm, 0, 0, kst)

	day := n
	if n.After(slotToday) {
		day = n.AddDate(0, 0, 1) // 이미 지남 → 내일
	}
	fcstDate = day.Format("20060102")
	return fcstDate, fcstTime
}

// SlotIsPast 는 특정 예보 시각(fcstDate="YYYYMMDD", fcstTime="HHmm")이 now(KST) 기준
// 이미 지났는지 판단한다. slot < now 이면 true. 동(同) 시각(slot == now)은 아직 안 지난
// 것으로 보고 false 를 반환한다(정각 딱 맞으면 표시). now 는 내부에서 KST 로 변환한다.
func SlotIsPast(now time.Time, fcstDate, fcstTime string) bool {
	n := now.In(kst)
	hh, mm := parseHHmm(fcstTime)
	slot := time.Date(yearOf(fcstDate), monthOf(fcstDate), dayOf(fcstDate), hh, mm, 0, 0, kst)
	return slot.Before(n)
}

// ultraRangeHours 는 초단기예보가 커버하는 예보 범위다(발표시점부터 +6시간). spec §4.1.
const ultraRangeHours = 6

// WithinUltraRange 는 특정 예보 시각(fcstDate="YYYYMMDD", fcstTime="HHmm")이 now(KST) 기준
// "지금부터 6시간 이내"인지 판단한다. 0 <= (slot - now) <= 6시간이면 true.
// 이 범위면 초단기예보(getUltraSrtFcst)를 우선 시도하고, 밖이면 단기예보를 쓴다(spec §4.1).
// 과거 시각(slot < now)은 초단기 범위로 보지 않는다(false). now 는 내부에서 KST 로 변환한다.
func WithinUltraRange(now time.Time, fcstDate, fcstTime string) bool {
	n := now.In(kst)
	hh, mm := parseHHmm(fcstTime)
	slot := time.Date(yearOf(fcstDate), monthOf(fcstDate), dayOf(fcstDate), hh, mm, 0, 0, kst)
	diff := slot.Sub(n)
	return diff >= 0 && diff <= ultraRangeHours*time.Hour
}

// parseHHmm 은 "HHmm" 을 시·분으로 분해한다. 형식이 아니면 0,0.
func parseHHmm(s string) (h, m int) {
	if len(s) != 4 {
		return 0, 0
	}
	for i := 0; i < 4; i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, 0
		}
	}
	h = int(s[0]-'0')*10 + int(s[1]-'0')
	m = int(s[2]-'0')*10 + int(s[3]-'0')
	return h, m
}

// HourlySlice 는 같은 VilageForecast 응답(items)에서 추가 호출 없이 시간별 흐름만 잘라낸다.
// 기준 시각(anchorDate "YYYYMMDD", anchorTime "HHmm" 정시)에서 before 시간 전 ~ after 시간 후
// 범위의 각 정시 슬롯을 HourlyPoint 로 만든다. 날짜 경계(자정 넘김/이전)도 처리한다.
// 데이터가 없는 슬롯은 건너뛴다(graceful — spec §9-2 폴백).
func HourlySlice(items []FcstItem, anchorDate, anchorTime string, before, after int) []HourlyPoint {
	hh, _ := parseHHmm(anchorTime)
	anchor := time.Date(yearOf(anchorDate), monthOf(anchorDate), dayOf(anchorDate), hh, 0, 0, 0, kst)

	out := make([]HourlyPoint, 0, before+after+1)
	for off := -before; off <= after; off++ {
		t := anchor.Add(time.Duration(off) * time.Hour)
		d := t.Format("20060102")
		tm := t.Format("1504")
		slot, ok := SlotForecastAt(items, d, tm)
		if !ok {
			continue
		}
		out = append(out, HourlyPoint{
			Time:    tm,
			TempC:   slot.TempC,
			PopPct:  slot.PopPct,
			PtyText: slot.PtyText,
		})
	}
	// off 를 -before..+after 로 순회하므로 out 은 이미 시간 오름차순이다(자정 넘김 포함).
	// "Time"(HHmm) 문자열 정렬은 날짜 경계에서 어긋나므로 추가 정렬하지 않는다.
	return out
}

// yearOf/monthOf/dayOf 는 "YYYYMMDD" 에서 연/월/일을 뽑는다. 형식이 아니면 0 처리되어
// 호출부에서 안전하게 빈 슬라이스로 귀결된다.
func yearOf(ymd string) int {
	if len(ymd) != 8 {
		return 0
	}
	return atoiDefault(ymd[:4], 0)
}

func monthOf(ymd string) time.Month {
	if len(ymd) != 8 {
		return time.January
	}
	return time.Month(atoiDefault(ymd[4:6], 1))
}

func dayOf(ymd string) int {
	if len(ymd) != 8 {
		return 1
	}
	return atoiDefault(ymd[6:8], 1)
}
