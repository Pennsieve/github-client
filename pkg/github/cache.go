package github

import "time"

const defaultTTL = 5 * time.Minute

type cacheItem struct {
	value     interface{}
	expiresAt time.Time
}

func (i cacheItem) isExpired() bool {
	return time.Now().After(i.expiresAt)
}

type cache interface {
	Add(key string, value interface{})
	Get(key string) (interface{}, bool)
	Remove(key string)
}

func newBasicCache(ttl time.Duration) *basicCache {
	return &basicCache{
		ttl:   ttl,
		cache: make(map[interface{}]cacheItem),
	}
}

type basicCache struct {
	ttl   time.Duration
	cache map[interface{}]cacheItem
}

func (c *basicCache) Add(key string, value interface{}) {
	c.cache[key] = cacheItem{value, time.Now().Add(c.ttl)}
}

func (c *basicCache) Get(key string) (interface{}, bool) {
	found, ok := c.cache[key]
	if ok && !found.isExpired() {
		return found.value, true
	}

	for k, i := range c.cache {
		if i.isExpired() {
			delete(c.cache, k)
		}
	}

	return nil, false
}

func (c *basicCache) Remove(key string) {
	delete(c.cache, key)
}
