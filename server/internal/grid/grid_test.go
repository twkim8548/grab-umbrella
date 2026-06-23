package grid

import "testing"

// 알려진 레퍼런스 값: 서울특별시 종로구 (lat 37.5727, lng 126.9806) → nx=60, ny=127
// (기상청 단기예보 격자표 기준)
func TestToGrid_Seoul(t *testing.T) {
	nx, ny := ToGrid(37.5727, 126.9806)
	if nx != 60 || ny != 127 {
		t.Fatalf("ToGrid(Seoul) = (%d,%d), want (60,127)", nx, ny)
	}
}
