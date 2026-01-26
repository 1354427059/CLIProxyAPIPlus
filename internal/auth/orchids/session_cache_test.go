package orchids

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSessionCache_Dedup(t *testing.T) {
	cache := NewSessionCache()
	calls := int64(0)
	fn := func() (any, error) {
		atomic.AddInt64(&calls, 1)
		time.Sleep(50 * time.Millisecond)
		return "ok", nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = cache.Do("orchids-1", fn)
		}()
	}
	wg.Wait()

	if atomic.LoadInt64(&calls) != 1 {
		t.Fatalf("expected singleflight to dedupe calls")
	}
}
