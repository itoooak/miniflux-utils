package content

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/itoooak/miniflux-utils/internal/cache"
	"github.com/itoooak/miniflux-utils/internal/miniflux"
)

type ContentFormat string

const (
	FormatHTML     ContentFormat = "html"
	FormatMarkdown ContentFormat = "markdown"
)

type ContentManager struct {
	minifluxClient  *miniflux.Client
	cache           cache.Cache
	converter       ContentConverter
	cacheKeyBuilder *cache.CacheKeyBuilder
}

type ContentMetadata struct {
	Length          int64
	ByteLength      int64
	TotalLength     int64
	TotalByteLength int64
	Encoding        string
	Format          ContentFormat
	ContentType     string
	FetchedAt       time.Time
}

func NewContentManager(
	minifluxClient *miniflux.Client,
	c cache.Cache,
	converter ContentConverter,
) (*ContentManager, error) {
	return &ContentManager{
		minifluxClient:  minifluxClient,
		cache:           c,
		converter:       converter,
		cacheKeyBuilder: cache.NewCacheKeyBuilder("content"),
	}, nil
}

func (cm *ContentManager) FetchContent(ctx context.Context, entryID int64, format ContentFormat, ttl time.Duration) ([]byte, *ContentMetadata, error) {
	var cacheKey string
	switch format {
	case FormatHTML:
		cacheKey = cm.cacheKeyBuilder.EntryHTML(entryID)
	case FormatMarkdown:
		cacheKey = cm.cacheKeyBuilder.EntryMarkdown(entryID)
	default:
		return nil, nil, fmt.Errorf("unsupported format: %s", format)
	}

	if cachedContent, ok := cm.cache.Get(cacheKey); ok {
		return cachedContent, cm.buildMetadata(cachedContent, format), nil
	}

	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	default:
	}

	entry, err := cm.minifluxClient.GetEntry(entryID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch entry: %w", err)
	}

	var content []byte
	switch format {
	case FormatHTML:
		if entry.Content == "" {
			return nil, nil, fmt.Errorf("entry has no content")
		}
		content = []byte(entry.Content)
	case FormatMarkdown:
		if entry.URL == "" {
			return nil, nil, fmt.Errorf("entry has no URL for Markdown conversion")
		}

		markdown, err := cm.converter.ConvertToMarkdown(ctx, entry.URL)
		if err != nil {
			fmt.Printf("warning: Markdown conversion failed, returning HTML: %v\n", err)
			if entry.Content != "" {
				content = []byte(entry.Content)
			} else {
				return nil, nil, fmt.Errorf("entry has no content and Markdown conversion failed")
			}
		} else {
			content = []byte(markdown)
		}
	}

	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	default:
	}

	if err := cm.cache.Set(cacheKey, content, ttl); err != nil {
		fmt.Printf("warning: failed to cache content: %v\n", err)
	}

	return content, cm.buildMetadata(content, format), nil
}

func (cm *ContentManager) GetContentRange(
	ctx context.Context,
	entryID int64,
	format ContentFormat,
	start, end int64,
	rangeType RangeType,
	ttl time.Duration,
) ([]byte, *ContentMetadata, error) {
	content, metadata, err := cm.FetchContent(ctx, entryID, format, ttl)
	if err != nil {
		return nil, nil, err
	}

	totalLength := metadata.TotalLength
	totalByteLength := metadata.TotalByteLength
	switch rangeType {
	case RangeTypeByte:
		if end > totalByteLength {
			end = totalByteLength
		}
	case RangeTypeChar:
		if end > totalLength {
			end = totalLength
		}
	default:
		return nil, nil, fmt.Errorf("unsupported range type: %s", rangeType)
	}

	rangeContent, err := ExtractRange(content, start, end, rangeType)
	if err != nil {
		return nil, nil, err
	}

	metadata.Length = int64(len(bytes.Runes(rangeContent)))
	metadata.ByteLength = int64(len(rangeContent))
	metadata.TotalLength = totalLength
	metadata.TotalByteLength = totalByteLength

	return rangeContent, metadata, nil
}

func (cm *ContentManager) buildMetadata(content []byte, format ContentFormat) *ContentMetadata {
	length := int64(len(bytes.Runes(content)))
	byteLength := int64(len(content))
	return &ContentMetadata{
		Length:          length,
		ByteLength:      byteLength,
		TotalLength:     length,
		TotalByteLength: byteLength,
		Encoding:        "utf-8",
		Format:          format,
		FetchedAt:       time.Now(),
	}
}

func (cm *ContentManager) GetContentMetadata(ctx context.Context, entryID int64, format ContentFormat) (*ContentMetadata, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	entry, err := cm.minifluxClient.GetEntry(entryID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch entry: %w", err)
	}

	var content []byte
	switch format {
	case FormatHTML:
		content = []byte(entry.Content)
	case FormatMarkdown:
		if entry.URL == "" {
			return nil, fmt.Errorf("entry has no URL for Markdown conversion")
		}

		markdown, err := cm.converter.ConvertToMarkdown(ctx, entry.URL)
		if err != nil {
			fmt.Printf("warning: Markdown conversion failed, returning HTML: %v\n", err)
			content = []byte(entry.Content)
		} else {
			content = []byte(markdown)
		}
	}

	length := int64(len(bytes.Runes(content)))
	byteLength := int64(len(content))
	return &ContentMetadata{
		Length:          length,
		ByteLength:      byteLength,
		TotalLength:     length,
		TotalByteLength: byteLength,
		Encoding:        "utf-8",
		Format:          format,
		ContentType:     "text/plain",
		FetchedAt:       time.Now(),
	}, nil
}

type RangeType string

const (
	RangeTypeChar RangeType = "char"
	RangeTypeByte RangeType = "byte"
)

func ExtractRange(content []byte, start, end int64, rangeType RangeType) ([]byte, error) {
	switch rangeType {
	case RangeTypeByte:
		if start < 0 || end < start || end > int64(len(content)) {
			return nil, fmt.Errorf("invalid byte range: start=%d, end=%d, length=%d", start, end, len(content))
		}
		return content[start:end], nil
	case RangeTypeChar:
		runes := bytes.Runes(content)
		if start < 0 || end < start || end > int64(len(runes)) {
			return nil, fmt.Errorf("invalid char range: start=%d, end=%d, length=%d", start, end, len(runes))
		}
		return []byte(string(runes[start:end])), nil
	default:
		return nil, fmt.Errorf("unsupported range type: %s", rangeType)
	}
}
