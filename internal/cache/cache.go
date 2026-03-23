package cache

import "time"

const DefaultTTL = 5 * time.Minute

type item struct {
	value     interface{}
	expiresAt time.Time
}

func (i item) isExpired() bool {
	return time.Now().After(i.expiresAt)
}

type Cache interface {
	Add(key string, value interface{})
	Get(key string) (interface{}, bool)
	Remove(key string)
}

func New(ttl time.Duration) Cache {
	return &basicCache{
		ttl:   ttl,
		cache: make(map[interface{}]item),
	}
}

type basicCache struct {
	ttl   time.Duration
	cache map[interface{}]item
}

func (c *basicCache) Add(key string, value interface{}) {
	c.cache[key] = item{value, time.Now().Add(c.ttl)}
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
