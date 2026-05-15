package cache

import (
	"fmt"
	"time"

	"github.com/itoooak/miniflux-utils/pkg/utils"
)

type Stats struct {
	TotalSize   int64
	ItemCount   int
	Hits        int64
	Misses      int64
	Evictions   int64
	LastCleanup time.Time
}

type Cache interface {
	Get(key string) ([]byte, bool)
	Set(key string, value []byte, ttl time.Duration) error
	Delete(key string) bool
	Cleanup() int
	Clear() error
	GetStats() Stats
	EnforceMaxSize(maxSize int64) error
}

type MultiCache struct {
	memoryCache *MemoryCache
	diskCache   *DiskCache
	maxSize     int64
	ttl         time.Duration
}

func NewCache(cfg *utils.CacheConfig) (Cache, error) {
	switch cfg.Mode {
	case utils.CacheMemory:
		return NewMemoryCache(), nil

	case utils.CacheDisk:
		return NewDiskCache(cfg.Path)

	case utils.CacheBoth:
		return NewMultiCache(cfg)

	default:
		return nil, fmt.Errorf("unknown cache mode: %s", cfg.Mode)
	}
}

func NewMultiCache(cfg *utils.CacheConfig) (*MultiCache, error) {
	diskCache, err := NewDiskCache(cfg.Path)
	if err != nil {
		return nil, err
	}

	return &MultiCache{
		memoryCache: NewMemoryCache(),
		diskCache:   diskCache,
		maxSize:     cfg.MaxSize,
		ttl:         cfg.TTL,
	}, nil
}

func (mc *MultiCache) Get(key string) ([]byte, bool) {
	if value, ok := mc.memoryCache.Get(key); ok {
		return value, true
	}

	if value, ok := mc.diskCache.Get(key); ok {
		_ = mc.memoryCache.Set(key, value, mc.ttl)
		return value, true
	}

	return nil, false
}

func (mc *MultiCache) Set(key string, value []byte, ttl time.Duration) error {
	if err := mc.memoryCache.Set(key, value, ttl); err != nil {
		return err
	}

	if err := mc.diskCache.Set(key, value, ttl); err != nil {
		fmt.Printf("warning: failed to store in disk cache: %v\n", err)
	}

	return nil
}

func (mc *MultiCache) Delete(key string) bool {
	memDeleted := mc.memoryCache.Delete(key)
	diskDeleted := mc.diskCache.Delete(key)
	return memDeleted || diskDeleted
}

func (mc *MultiCache) Cleanup() int {
	memCleaned := mc.memoryCache.Cleanup()
	diskCleaned := mc.diskCache.Cleanup()
	return memCleaned + diskCleaned
}

func (mc *MultiCache) Clear() error {
	_ = mc.memoryCache.Clear()
	return mc.diskCache.Clear()
}

func (mc *MultiCache) GetStats() Stats {
	memStats := mc.memoryCache.GetStats()
	diskStats := mc.diskCache.GetStats()

	return Stats{
		TotalSize:   memStats.TotalSize + diskStats.TotalSize,
		ItemCount:   memStats.ItemCount + diskStats.ItemCount,
		Hits:        memStats.Hits + diskStats.Hits,
		Misses:      memStats.Misses + diskStats.Misses,
		Evictions:   memStats.Evictions + diskStats.Evictions,
		LastCleanup: memStats.LastCleanup,
	}
}

func (mc *MultiCache) EnforceMaxSize(maxSize int64) error {
	_ = mc.memoryCache.EnforceMaxSize(maxSize / 2)

	return mc.diskCache.EnforceMaxSize(maxSize / 2)
}

type CacheKeyBuilder struct {
	prefix string
}

func NewCacheKeyBuilder(prefix string) *CacheKeyBuilder {
	return &CacheKeyBuilder{prefix: prefix}
}

// EntryHTML builds a key for HTML content of an entry
func (b *CacheKeyBuilder) EntryHTML(entryID int64) string {
	return fmt.Sprintf("%s:entry:%d:html", b.prefix, entryID)
}

// EntryMarkdown builds a key for Markdown content of an entry
func (b *CacheKeyBuilder) EntryMarkdown(entryID int64) string {
	return fmt.Sprintf("%s:entry:%d:md", b.prefix, entryID)
}

// EntryMetadata builds a key for entry metadata
func (b *CacheKeyBuilder) EntryMetadata(entryID int64) string {
	return fmt.Sprintf("%s:entry:%d:metadata", b.prefix, entryID)
}

// FeedsListKey builds a key for feeds list
func (b *CacheKeyBuilder) FeedsListKey() string {
	return fmt.Sprintf("%s:feeds:list", b.prefix)
}

// CategoriesListKey builds a key for categories list
func (b *CacheKeyBuilder) CategoriesListKey() string {
	return fmt.Sprintf("%s:categories:list", b.prefix)
}
