package weather

import (
	"sync"
	"testing"
)

func TestForecastCacheHit(t *testing.T) {
	c := newForecastCache()
	items := []FcstItem{{Category: "TMP", FcstValue: "23"}}

	if _, ok := c.get(kindVilage, 60, 127, "20260623", "0800"); ok {
		t.Fatal("expected miss before put")
	}
	c.put(kindVilage, 60, 127, "20260623", "0800", items)

	got, ok := c.get(kindVilage, 60, 127, "20260623", "0800")
	if !ok {
		t.Fatal("expected hit after put")
	}
	if len(got) != 1 || got[0].FcstValue != "23" {
		t.Errorf("cached items = %+v", got)
	}

	// 다른 격자는 미스.
	if _, ok := c.get(kindVilage, 61, 127, "20260623", "0800"); ok {
		t.Error("expected miss for different grid")
	}
	// 다른 발표본은 미스.
	if _, ok := c.get(kindVilage, 60, 127, "20260623", "1100"); ok {
		t.Error("expected miss for different release")
	}
}

// TestForecastCacheKindSeparation 은 같은 격자·같은 발표본이라도 단기/초단기 종류가 다르면
// 캐시키가 충돌하지 않음을 검증한다(spec §4.6).
func TestForecastCacheKindSeparation(t *testing.T) {
	c := newForecastCache()
	c.put(kindVilage, 60, 127, "20260623", "0800", []FcstItem{{FcstValue: "vilage"}})
	c.put(kindUltra, 60, 127, "20260623", "0800", []FcstItem{{FcstValue: "ultra"}})

	if got, ok := c.get(kindVilage, 60, 127, "20260623", "0800"); !ok || got[0].FcstValue != "vilage" {
		t.Errorf("vilage entry overwritten by ultra: ok=%v %+v", ok, got)
	}
	if got, ok := c.get(kindUltra, 60, 127, "20260623", "0800"); !ok || got[0].FcstValue != "ultra" {
		t.Errorf("ultra entry missing or wrong: ok=%v %+v", ok, got)
	}
}

func TestForecastCacheEvictsOldRelease(t *testing.T) {
	c := newForecastCache()
	c.put(kindVilage, 60, 127, "20260623", "0500", []FcstItem{{FcstValue: "old"}})
	c.put(kindVilage, 60, 127, "20260623", "0800", []FcstItem{{FcstValue: "new"}})

	// 같은 격자의 이전 발표본(0500)은 정리되어야 한다.
	if _, ok := c.get(kindVilage, 60, 127, "20260623", "0500"); ok {
		t.Error("expected old release evicted")
	}
	// 최신 발표본(0800)은 유지.
	if got, ok := c.get(kindVilage, 60, 127, "20260623", "0800"); !ok || got[0].FcstValue != "new" {
		t.Errorf("expected latest release retained, got ok=%v %+v", ok, got)
	}

	// 다른 격자는 영향 없음.
	c.put(kindVilage, 99, 99, "20260623", "0500", []FcstItem{{FcstValue: "other"}})
	c.put(kindVilage, 60, 127, "20260623", "1100", []FcstItem{{FcstValue: "newer"}})
	if _, ok := c.get(kindVilage, 99, 99, "20260623", "0500"); !ok {
		t.Error("different grid should not be evicted by another grid's put")
	}
}

func TestForecastCacheConcurrent(t *testing.T) {
	c := newForecastCache()
	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			// 동일 격자/발표본에 동시 put/get — race detector 로 안전성 검증.
			c.put(kindVilage, 60, 127, "20260623", "0800", []FcstItem{{FcstValue: "v"}})
			_, _ = c.get(kindVilage, 60, 127, "20260623", "0800")
			// 격자 다양화로 eviction 경로도 동시에 친다.
			c.put(kindVilage, i, i, "20260623", "0800", []FcstItem{{FcstValue: "g"}})
		}(i)
	}
	wg.Wait()

	if _, ok := c.get(kindVilage, 60, 127, "20260623", "0800"); !ok {
		t.Error("expected shared key present after concurrent access")
	}
}
