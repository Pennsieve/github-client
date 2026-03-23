package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	c := New(DefaultTTL)
	assert.NotNil(t, c)
}

func TestAddAndGet(t *testing.T) {
	c := New(1 * time.Minute)

	c.Add("key1", "value1")
	c.Add("key2", 42)

	val, ok := c.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, "value1", val)

	val, ok = c.Get("key2")
	assert.True(t, ok)
	assert.Equal(t, 42, val)
}

func TestGetMiss(t *testing.T) {
	c := New(1 * time.Minute)

	val, ok := c.Get("nonexistent")
	assert.False(t, ok)
	assert.Nil(t, val)
}

func TestGetExpired(t *testing.T) {
	c := New(1 * time.Millisecond)

	c.Add("key", "value")
	time.Sleep(5 * time.Millisecond)

	val, ok := c.Get("key")
	assert.False(t, ok)
	assert.Nil(t, val)
}

func TestExpiredEntriesCleanedOnGet(t *testing.T) {
	c := New(1 * time.Millisecond)
	bc := c.(*basicCache)

	c.Add("expired1", "a")
	c.Add("expired2", "b")
	time.Sleep(5 * time.Millisecond)

	_, _ = c.Get("anything")

	assert.Empty(t, bc.cache)
}

func TestRemove(t *testing.T) {
	c := New(1 * time.Minute)

	c.Add("key", "value")
	c.Remove("key")

	val, ok := c.Get("key")
	assert.False(t, ok)
	assert.Nil(t, val)
}

func TestRemoveNonexistent(t *testing.T) {
	c := New(1 * time.Minute)
	c.Remove("nonexistent")
}

func TestOverwrite(t *testing.T) {
	c := New(1 * time.Minute)

	c.Add("key", "first")
	c.Add("key", "second")

	val, ok := c.Get("key")
	assert.True(t, ok)
	assert.Equal(t, "second", val)
}
