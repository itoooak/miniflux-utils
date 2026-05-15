package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type DiskCache struct {
	mu       sync.RWMutex
	basePath string
	stats    *CacheStats
}

type CacheFileMetadata struct {
	Key        string    `json:"key"`
	ExpiresAt  time.Time `json:"expires_at"`
	CreatedAt  time.Time `json:"created_at"`
	AccessedAt time.Time `json:"accessed_at"`
	Size       int64     `json:"size"`
}

func NewDiskCache(basePath string) (*DiskCache, error) {
	if err := os.MkdirAll(basePath, 0750); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &DiskCache{
		basePath: basePath,
		stats: &CacheStats{
			LastCleanup: time.Now(),
		},
	}, nil
}

func (dc *DiskCache) Get(key string) ([]byte, bool) {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	filePath := dc.getFilePath(key)
	metadataPath := dc.getMetadataPath(key)

	metadata, err := dc.readMetadata(metadataPath)
	if err != nil {
		dc.stats.recordMiss()
		return nil, false
	}

	if time.Now().After(metadata.ExpiresAt) {
		dc.stats.recordMiss()
		return nil, false
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		dc.stats.recordMiss()
		return nil, false
	}

	metadata.AccessedAt = time.Now()
	_ = dc.writeMetadata(metadataPath, metadata)

	dc.stats.recordHit()
	return data, true
}

func (dc *DiskCache) Set(key string, value []byte, ttl time.Duration) error {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	filePath := dc.getFilePath(key)
	metadataPath := dc.getMetadataPath(key)

	if err := os.WriteFile(filePath, value, 0600); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	metadata := &CacheFileMetadata{
		Key:       key,
		ExpiresAt: time.Now().Add(ttl),
		CreatedAt: time.Now(),
		Size:      int64(len(value)),
	}

	if err := dc.writeMetadata(metadataPath, metadata); err != nil {
		_ = os.Remove(filePath)
		return err
	}

	dc.stats.TotalSize += metadata.Size
	dc.stats.ItemCount++

	return nil
}

func (dc *DiskCache) Delete(key string) bool {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	filePath := dc.getFilePath(key)
	metadataPath := dc.getMetadataPath(key)

	metadata, err := dc.readMetadata(metadataPath)
	if err != nil {
		return false
	}

	_ = os.Remove(filePath)
	_ = os.Remove(metadataPath)

	dc.stats.TotalSize -= metadata.Size
	dc.stats.ItemCount--

	return true
}

func (dc *DiskCache) Cleanup() int {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	removed := 0
	now := time.Now()

	metadataDir := filepath.Join(dc.basePath, "metadata")
	if _, err := os.Stat(metadataDir); os.IsNotExist(err) {
		dc.stats.LastCleanup = now
		return removed
	}

	entries, err := os.ReadDir(metadataDir)
	if err != nil {
		dc.stats.LastCleanup = now
		return removed
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		metadataPath := filepath.Join(metadataDir, entry.Name())
		metadata, err := dc.readMetadata(metadataPath)
		if err != nil {
			continue
		}

		if now.After(metadata.ExpiresAt) {
			key := metadata.Key
			filePath := dc.getFilePath(key)
			_ = os.Remove(filePath)
			_ = os.Remove(metadataPath)
			dc.stats.TotalSize -= metadata.Size
			dc.stats.ItemCount--
			removed++
		}
	}

	dc.stats.LastCleanup = now
	dc.stats.Evictions += int64(removed)

	return removed
}

func (dc *DiskCache) Clear() error {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	if err := os.RemoveAll(dc.basePath); err != nil {
		return err
	}

	if err := os.MkdirAll(dc.basePath, 0750); err != nil {
		return err
	}

	dc.stats.TotalSize = 0
	dc.stats.ItemCount = 0

	return nil
}

func (dc *DiskCache) GetStats() Stats {
	dc.stats.mu.RLock()
	defer dc.stats.mu.RUnlock()

	return Stats{
		TotalSize:   dc.stats.TotalSize,
		ItemCount:   dc.stats.ItemCount,
		Hits:        dc.stats.Hits,
		Misses:      dc.stats.Misses,
		Evictions:   dc.stats.Evictions,
		LastCleanup: dc.stats.LastCleanup,
	}
}

func (dc *DiskCache) EnforceMaxSize(maxSize int64) error {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	if dc.stats.TotalSize <= maxSize {
		return nil
	}

	metadataDir := filepath.Join(dc.basePath, "metadata")
	entries, err := os.ReadDir(metadataDir)
	if err != nil {
		return err
	}

	type metadataWithPath struct {
		path     string
		metadata *CacheFileMetadata
	}

	var items []metadataWithPath
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		metadataPath := filepath.Join(metadataDir, entry.Name())
		metadata, err := dc.readMetadata(metadataPath)
		if err != nil {
			continue
		}

		items = append(items, metadataWithPath{metadataPath, metadata})
	}

	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[i].metadata.CreatedAt.After(items[j].metadata.CreatedAt) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}

	for _, item := range items {
		if dc.stats.TotalSize <= maxSize {
			break
		}

		filePath := dc.getFilePath(item.metadata.Key)
		_ = os.Remove(filePath)
		_ = os.Remove(item.path)
		dc.stats.TotalSize -= item.metadata.Size
		dc.stats.ItemCount--
		dc.stats.Evictions++
	}

	return nil
}

func (dc *DiskCache) getFilePath(key string) string {
	hash := sha256.Sum256([]byte(key))
	hashStr := hex.EncodeToString(hash[:])
	return filepath.Join(dc.basePath, "data", hashStr)
}

func (dc *DiskCache) getMetadataPath(key string) string {
	hash := sha256.Sum256([]byte(key))
	hashStr := hex.EncodeToString(hash[:])
	return filepath.Join(dc.basePath, "metadata", hashStr+".json")
}

func (dc *DiskCache) readMetadata(path string) (*CacheFileMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var metadata CacheFileMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}

	return &metadata, nil
}

func (dc *DiskCache) writeMetadata(path string, metadata *CacheFileMetadata) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}

	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

func (dc *DiskCache) Export(outputPath string) (err error) {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, file.Close()) }()

	return nil
}

func (dc *DiskCache) Import(inputPath string) (err error) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	file, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, file.Close()) }()

	_, err = io.Copy(os.Stdout, file)
	return err
}
