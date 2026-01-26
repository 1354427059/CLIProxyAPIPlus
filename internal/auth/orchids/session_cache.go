package orchids

import (
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

type SessionCache struct {
	group       singleflight.Group
	mu          sync.Mutex
	lastRefresh map[string]time.Time
}

func NewSessionCache() *SessionCache {
	return &SessionCache{lastRefresh: make(map[string]time.Time)}
}

func (c *SessionCache) CanRefresh(key string, minInterval time.Duration) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	last := c.lastRefresh[key]
	if !last.IsZero() && time.Since(last) < minInterval {
		return false
	}
	c.lastRefresh[key] = time.Now()
	return true
}

func (c *SessionCache) Do(key string, fn func() (any, error)) (any, error) {
	value, err, _ := c.group.Do(key, fn)
	return value, err
}
