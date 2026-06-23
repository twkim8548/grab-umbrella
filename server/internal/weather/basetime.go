package weather

import "time"

// kst 는 한국 표준시(Asia/Seoul, UTC+9) 고정 오프셋이다.
// tzdata 가 없는 환경에서도 동작하도록 LoadLocation 대신 FixedZone 을 쓴다.
var kst = time.FixedZone("KST", 9*60*60)

// vilageReleaseHours 는 단기예보(getVilageFcst) 발표시각이다. 1일 8회, 매 3시간.
var vilageReleaseHours = []int{2, 5, 8, 11, 14, 17, 20, 23}

// vilageSafetyMargin 은 발표 정각 이후 데이터가 실제 제공되기까지의 안전 마진이다.
// 공식 가이드(단기예보_활용가이드_260623.docx)는 단기예보 API 제공 시간을
// "발표 후 10분 이후"(02:10, 05:10, 08:10 …)로 명시한다. 정각 직후 호출하면
// 빈 응답이 오므로, "발표 후 이 시간만큼 지난" 가장 최근 발표본을 선택한다.
// 가이드값 10분에 생성/네트워크 여유를 더해 15분으로 둔다.
// (참고: 45분은 초단기예보 getUltraSrtFcst 의 마진이며 단기예보엔 과도하다.)
const vilageSafetyMargin = 15 * time.Minute

// BaseTime 은 단기예보(getVilageFcst) 호출용 base_date/base_time 을 계산한다. spec §4.5.
//
// 안전 마진 규칙: now(KST) 기준으로 "발표 정각 + vilageSafetyMargin(15분)"을 이미 지난
// 가장 최근 발표본을 선택한다. 예) 08:10 → 08시 발표본은 아직 15분이 안 지났으므로 05시
// 발표본 사용. 08:20 → 08시 발표본 사용 가능. 02시 발표 전(00:00~02:14)이면 전날 23시
// 발표본을 사용하므로 base_date 가 어제로 넘어간다.
//
// 반환 형식: baseDate="YYYYMMDD", baseTime="HHmm"(예: "2300").
func BaseTime(now time.Time) (baseDate, baseTime string) {
	n := now.In(kst)

	// now 시점에 "이미 마진을 넘긴" 가장 최근 발표시각을 오늘 발표본들 중에서 찾는다.
	chosenHour := -1
	for _, h := range vilageReleaseHours {
		release := time.Date(n.Year(), n.Month(), n.Day(), h, 0, 0, 0, kst)
		if !n.Before(release.Add(vilageSafetyMargin)) {
			chosenHour = h
		}
	}

	day := n
	if chosenHour == -1 {
		// 오늘 02시 발표본조차 마진을 못 넘김(00:00~02:44) → 전날 마지막 발표본(23시).
		day = n.AddDate(0, 0, -1)
		chosenHour = vilageReleaseHours[len(vilageReleaseHours)-1]
	}

	baseDate = day.Format("20060102")
	baseTime = time.Date(day.Year(), day.Month(), day.Day(), chosenHour, 0, 0, 0, kst).Format("1504")
	return baseDate, baseTime
}
