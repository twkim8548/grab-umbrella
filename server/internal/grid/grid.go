// Package grid 은 위경도(lat,lng)를 기상청 단기예보 격자(nx,ny)로 변환한다.
// LCC DFS(Lambert Conformal Conic) 좌표 변환. spec §8: v1 composables/grid.ts 포팅.
package grid

import "math"

// 기상청 단기예보 격자 변환 상수 (LCC DFS)
const (
	re      = 6371.00877 // 지구 반경 (km)
	grid    = 5.0        // 격자 간격 (km)
	slat1   = 30.0       // 표준 위도 1 (deg)
	slat2   = 60.0       // 표준 위도 2 (deg)
	olon    = 126.0      // 기준점 경도 (deg)
	olat    = 38.0       // 기준점 위도 (deg)
	xo      = 43.0       // 기준점 X 좌표 (격자)
	yo      = 136.0      // 기준점 Y 좌표 (격자)
	degrad  = math.Pi / 180.0
)

// ToGrid 는 위경도를 격자 좌표(nx, ny)로 변환한다.
func ToGrid(lat, lng float64) (nx, ny int) {
	reGrid := re / grid
	slat1Rad := slat1 * degrad
	slat2Rad := slat2 * degrad
	olonRad := olon * degrad
	olatRad := olat * degrad

	sn := math.Tan(math.Pi*0.25+slat2Rad*0.5) / math.Tan(math.Pi*0.25+slat1Rad*0.5)
	sn = math.Log(math.Cos(slat1Rad)/math.Cos(slat2Rad)) / math.Log(sn)
	sf := math.Tan(math.Pi*0.25 + slat1Rad*0.5)
	sf = math.Pow(sf, sn) * math.Cos(slat1Rad) / sn
	ro := math.Tan(math.Pi*0.25 + olatRad*0.5)
	ro = reGrid * sf / math.Pow(ro, sn)

	ra := math.Tan(math.Pi*0.25 + lat*degrad*0.5)
	ra = reGrid * sf / math.Pow(ra, sn)
	theta := lng*degrad - olonRad
	if theta > math.Pi {
		theta -= 2.0 * math.Pi
	}
	if theta < -math.Pi {
		theta += 2.0 * math.Pi
	}
	theta *= sn

	nx = int(math.Floor(ra*math.Sin(theta) + xo + 0.5))
	ny = int(math.Floor(ro - ra*math.Cos(theta) + yo + 0.5))
	return nx, ny
}
