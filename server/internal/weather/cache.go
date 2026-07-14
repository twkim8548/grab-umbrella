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
// 예보 종류(kind). 단기예보와 초단기예보는 같은 격자라도 발표본·카테고리가 다르므로
// 캐시키를 반드시 구분해야 한다(spec §4.6).
const (
	kindVilage = "vilage"
	kindUltra  = "ultra"
)

type forecastCache struct {
	mu      sync.Mutex
	entries map[string][]FcstItem // key = "kind:nx:ny:baseDate:baseTime"
	latest  map[string]string     // grid prefix -> latest "baseDate:baseTime"
}

func newForecastCache() *forecastCache {
	return &forecastCache{
		entries: make(map[string][]FcstItem),
		latest:  make(map[string]string),
	}
}

// cacheKey 는 종류·격자·발표본을 캐시키로 합성한다.
func cacheKey(kind string, nx, ny int, baseDate, baseTime string) string {
	return kind + ":" + strconv.Itoa(nx) + ":" + strconv.Itoa(ny) + ":" + baseDate + ":" + baseTime
}

// gridPrefix 는 한 종류·격자의 모든 발표본 키 공통 접두사다("kind:nx:ny:").
func gridPrefix(kind string, nx, ny int) string {
	return kind + ":" + strconv.Itoa(nx) + ":" + strconv.Itoa(ny) + ":"
}

// get 은 캐시 적중 시 항목과 true 를 반환한다.
func (c *forecastCache) get(kind string, nx, ny int, baseDate, baseTime string) ([]FcstItem, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	items, ok := c.entries[cacheKey(kind, nx, ny, baseDate, baseTime)]
	return items, ok
}

// put 은 항목을 저장하고, 같은 종류·격자의 다른(오래된) 발표본 키를 제거한다.
func (c *forecastCache) put(kind string, nx, ny int, baseDate, baseTime string, items []FcstItem) {
	key := cacheKey(kind, nx, ny, baseDate, baseTime)
	prefix := gridPrefix(kind, nx, ny)
	release := baseDate + ":" + baseTime

	c.mu.Lock()
	defer c.mu.Unlock()
	// 발표 경계에서 이전 발표본 요청이 늦게 끝나더라도 이미 저장된 새 발표본을
	// 지우거나 덮지 않는다.
	if latest, ok := c.latest[prefix]; ok && release < latest {
		return
	}
	// 같은 격자의 이전 발표본 정리(격자당 최신 1개만 유지).
	for k := range c.entries {
		if k != key && strings.HasPrefix(k, prefix) {
			delete(c.entries, k)
		}
	}
	c.entries[key] = items
	c.latest[prefix] = release
}
