package cache

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

type Item struct {
	Value     interface{} `json:"value"`
	ExpiresAt time.Time   `json:"expires_at"`
}

type MemoryCache struct {
	mu    sync.RWMutex
	items map[string]Item
	ttl   time.Duration
}

func NewMemoryCache(ttl time.Duration) *MemoryCache {
	c := &MemoryCache{
		items: make(map[string]Item),
		ttl:   ttl,
	}
	go c.cleanupLoop()
	return c
}

func (c *MemoryCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, ok := c.items[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(item.ExpiresAt) {
		return nil, false
	}
	return item.Value, true
}

func (c *MemoryCache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = Item{
		Value:     value,
		ExpiresAt: time.Now().Add(c.ttl),
	}
}

func (c *MemoryCache) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = Item{
		Value:     value,
		ExpiresAt: time.Now().Add(ttl),
	}
}

func (c *MemoryCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

func (c *MemoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]Item)
}

func (c *MemoryCache) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for k, v := range c.items {
			if now.After(v.ExpiresAt) {
				delete(c.items, k)
			}
		}
		c.mu.Unlock()
	}
}

func Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

type Store interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

type TwoLayerCache struct {
	mem   *MemoryCache
	store Store
	ttl   time.Duration
}

func NewTwoLayerCache(mem *MemoryCache, store Store, ttl time.Duration) *TwoLayerCache {
	return &TwoLayerCache{mem: mem, store: store, ttl: ttl}
}

func (c *TwoLayerCache) Get(ctx context.Context, key string) (interface{}, bool, error) {
	if v, ok := c.mem.Get(key); ok {
		return v, true, nil
	}

	data, ok, err := c.store.Get(ctx, key)
	if err != nil || !ok {
		return nil, false, err
	}

	var v interface{}
	if err := Unmarshal(data, &v); err != nil {
		return nil, false, err
	}

	c.mem.Set(key, v)
	return v, true, nil
}

func (c *TwoLayerCache) Set(ctx context.Context, key string, value interface{}) error {
	c.mem.Set(key, value)

	data, err := Marshal(value)
	if err != nil {
		return err
	}

	return c.store.Set(ctx, key, data, c.ttl)
}

func (c *TwoLayerCache) Delete(key string) {
	c.mem.Delete(key)
}
