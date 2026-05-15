package cache

import (
	"sync"
	"time"
)

type MemoryCache struct {
	mu    sync.RWMutex
	items map[string]*CacheItem
	stats *CacheStats
}

// CacheItem represents a cached item with metadata
type CacheItem struct {
	Value      []byte
	ExpiresAt  time.Time
	CreatedAt  time.Time
	AccessedAt time.Time
	Size       int64
}

// CacheStats holds cache statistics
type CacheStats struct {
	mu          sync.RWMutex
	TotalSize   int64
	ItemCount   int
	Hits        int64
	Misses      int64
	Evictions   int64
	LastCleanup time.Time
}

func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		items: make(map[string]*CacheItem),
		stats: &CacheStats{
			LastCleanup: time.Now(),
		},
	}
}

func (mc *MemoryCache) Get(key string) ([]byte, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	item, exists := mc.items[key]
	if !exists {
		mc.stats.recordMiss()
		return nil, false
	}

	if time.Now().After(item.ExpiresAt) {
		mc.stats.recordMiss()
		return nil, false
	}

	item.AccessedAt = time.Now()
	mc.stats.recordHit()
	return item.Value, true
}

func (mc *MemoryCache) Set(key string, value []byte, ttl time.Duration) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	size := int64(len(value))
	expiresAt := time.Now().Add(ttl)

	if oldItem, exists := mc.items[key]; exists {
		mc.stats.TotalSize -= oldItem.Size
		mc.stats.ItemCount--
	}

	mc.items[key] = &CacheItem{
		Value:      value,
		ExpiresAt:  expiresAt,
		CreatedAt:  time.Now(),
		AccessedAt: time.Now(),
		Size:       size,
	}

	mc.stats.TotalSize += size
	mc.stats.ItemCount++

	return nil
}

func (mc *MemoryCache) Delete(key string) bool {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if item, exists := mc.items[key]; exists {
		mc.stats.TotalSize -= item.Size
		mc.stats.ItemCount--
		delete(mc.items, key)
		return true
	}
	return false
}

func (mc *MemoryCache) Cleanup() int {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	now := time.Now()
	removed := 0

	for key, item := range mc.items {
		if now.After(item.ExpiresAt) {
			mc.stats.TotalSize -= item.Size
			mc.stats.ItemCount--
			delete(mc.items, key)
			removed++
		}
	}

	mc.stats.LastCleanup = now
	mc.stats.Evictions += int64(removed)

	return removed
}

func (mc *MemoryCache) Clear() error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.items = make(map[string]*CacheItem)
	mc.stats.TotalSize = 0
	mc.stats.ItemCount = 0
	return nil
}

func (mc *MemoryCache) GetStats() Stats {
	mc.stats.mu.RLock()
	defer mc.stats.mu.RUnlock()

	return Stats{
		TotalSize:   mc.stats.TotalSize,
		ItemCount:   mc.stats.ItemCount,
		Hits:        mc.stats.Hits,
		Misses:      mc.stats.Misses,
		Evictions:   mc.stats.Evictions,
		LastCleanup: mc.stats.LastCleanup,
	}
}

func (mc *MemoryCache) EnforceMaxSize(maxSize int64) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if mc.stats.TotalSize <= maxSize {
		return nil
	}

	type itemWithKey struct {
		key  string
		item *CacheItem
	}

	var items []itemWithKey
	for k, item := range mc.items {
		items = append(items, itemWithKey{k, item})
	}

	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[i].item.AccessedAt.After(items[j].item.AccessedAt) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}

	for _, item := range items {
		if mc.stats.TotalSize <= maxSize {
			break
		}

		mc.stats.TotalSize -= item.item.Size
		mc.stats.ItemCount--
		mc.stats.Evictions++
		delete(mc.items, item.key)
	}

	return nil
}

func (s *CacheStats) recordHit() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Hits++
}

func (s *CacheStats) recordMiss() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Misses++
}
