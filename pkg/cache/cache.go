package cache

import (
	"sync"
)

type Cache[T any] struct {
	cache map[string][]T
	mutex sync.RWMutex
}

func NewCache[T any]() *Cache[T] {
	return &Cache[T]{
		cache: make(map[string][]T),
	}
}

func (c *Cache[T]) Add(key string, item T) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	// No need to check if the key exists; append works fine either way
	c.cache[key] = append(c.cache[key], item)
}

func (c *Cache[T]) Remove(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	delete(c.cache, key)
}

func (c *Cache[T]) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.cache = make(map[string][]T)
}

func (c *Cache[T]) Get(key string) []T {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	if value, exists := c.cache[key]; exists {
		return value
	}
	return []T{}
}
