package weather

import (
	"strconv"
	"strings"
	"sync"
)

// forecastCache 는 단기예보 응답을 격자·발표본 단위로 메모리에 캐싱한다(spec §4.6).
// 같은 격자(nx,ny) + 같은 발표본(baseDate,baseTime)은 1회만 기상청을 호출하고 재사용한다.
// 같은 동네 사용자가 여럿이어도 호출은 1번으로 공유된다.
//
// 만료: 발표본이 바뀌면 캐시키가 바뀌어 자연 무효화된다(TTL 불필요). 다만 오래된 발표본
// 키가 누적되어 메모리가 무한 증가하는 것을 막기 위해, 새 발표본이 들어오면 그 격자의
// 이전 발표본 항목들을 정리한다(격자당 최신 발표본 1개만 유지).
type forecastCache struct {
	mu      sync.Mutex
	entries map[string][]FcstItem // key = "nx:ny:baseDate:baseTime"
}

func newForecastCache() *forecastCache {
	return &forecastCache{entries: make(map[string][]FcstItem)}
}

// cacheKey 는 격자·발표본을 캐시키로 합성한다.
func cacheKey(nx, ny int, baseDate, baseTime string) string {
	return strconv.Itoa(nx) + ":" + strconv.Itoa(ny) + ":" + baseDate + ":" + baseTime
}

// gridPrefix 는 한 격자의 모든 발표본 키 공통 접두사다("nx:ny:").
func gridPrefix(nx, ny int) string {
	return strconv.Itoa(nx) + ":" + strconv.Itoa(ny) + ":"
}

// get 은 캐시 적중 시 항목과 true 를 반환한다.
func (c *forecastCache) get(nx, ny int, baseDate, baseTime string) ([]FcstItem, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	items, ok := c.entries[cacheKey(nx, ny, baseDate, baseTime)]
	return items, ok
}

// put 은 항목을 저장하고, 같은 격자의 다른(오래된) 발표본 키를 제거한다.
func (c *forecastCache) put(nx, ny int, baseDate, baseTime string, items []FcstItem) {
	key := cacheKey(nx, ny, baseDate, baseTime)
	prefix := gridPrefix(nx, ny)

	c.mu.Lock()
	defer c.mu.Unlock()
	// 같은 격자의 이전 발표본 정리(격자당 최신 1개만 유지).
	for k := range c.entries {
		if k != key && strings.HasPrefix(k, prefix) {
			delete(c.entries, k)
		}
	}
	c.entries[key] = items
}
