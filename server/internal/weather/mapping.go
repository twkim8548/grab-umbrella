package weather

// category 매핑. spec §8 (v1 composables/weather.ts 검증된 매핑).

// popUmbrellaThreshold 는 강수확률(POP) 우산 권장 임계값(%)이다.
// PTY 가 없음(0)이어도 POP 가 이 값 이상이면 우산을 권장한다.
const popUmbrellaThreshold = 60

// skyText 는 SKY 코드(1/3/4)를 한글 텍스트로 변환한다.
func skyText(code int) string {
	switch code {
	case 1:
		return "맑음"
	case 3:
		return "구름많음"
	case 4:
		return "흐림"
	default:
		return ""
	}
}

// ptyText 는 PTY 코드(0~4)를 한글 텍스트로 변환한다.
func ptyText(code int) string {
	switch code {
	case 0:
		return "없음"
	case 1:
		return "비"
	case 2:
		return "비/눈"
	case 3:
		return "눈"
	case 4:
		return "소나기"
	default:
		return ""
	}
}

// NeedUmbrella 는 강수형태(PTY)와 강수확률(POP)로 우산 필요 여부를 판정한다. spec §8.
// PTY != 0 (비/눈/소나기 등)이거나 POP 가 임계값 이상이면 true.
func NeedUmbrella(pty, pop int) bool {
	return pty != 0 || pop >= popUmbrellaThreshold
}
