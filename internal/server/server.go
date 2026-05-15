package server

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/itoooak/miniflux-utils/internal/cache"
	"github.com/itoooak/miniflux-utils/internal/content"
	"github.com/itoooak/miniflux-utils/internal/miniflux"
	"github.com/itoooak/miniflux-utils/pkg/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	minifluxclient "miniflux.app/v2/client"
)

type Server struct {
	config         *utils.Config
	minifluxClient *miniflux.Client
	cacheManager   cache.Cache
	contentManager *content.ContentManager
	mcpServer      *mcp.Server
	logger         *log.Logger
}

func NewServer(cfg *utils.Config, logger *log.Logger) (*Server, error) {
	minifluxClient, err := miniflux.NewClient(&cfg.Miniflux)
	if err != nil {
		return nil, fmt.Errorf("failed to create Miniflux client: %w", err)
	}

	cacheManager, err := cache.NewCache(&cfg.Cache)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cache: %w", err)
	}

	converter := content.NewJinaConverter(cfg.Converter.Timeout)

	contentManager, err := content.NewContentManager(minifluxClient, cacheManager, converter)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize content manager: %w", err)
	}

	mcpSrv := mcp.NewServer(&mcp.Implementation{
		Name:    "miniflux-mcp",
		Version: "0.1.0",
	}, nil)

	server := &Server{
		config:         cfg,
		minifluxClient: minifluxClient,
		cacheManager:   cacheManager,
		contentManager: contentManager,
		mcpServer:      mcpSrv,
		logger:         logger,
	}

	server.registerTools()

	return server, nil
}

func (s *Server) registerTools() {
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "list_feeds",
		Description: "Get a list of all feeds (summary only)",
	}, s.listFeedsHandler)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_feed",
		Description: "Get a specific feed by ID (summary only)",
	}, s.getFeedHandler)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_feed_entries",
		Description: "Get entries from a specific feed (metadata only, no content)",
	}, s.getFeedEntriesHandler)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "list_entries",
		Description: "Get a list of all entries with optional filters (metadata only, no content)",
	}, s.listEntriesHandler)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_entry",
		Description: "Get a specific entry by ID (metadata only, no content)",
	}, s.getEntryHandler)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_entry_content_full",
		Description: "Fetch full entry content (discouraged because the response size can be large; prefer get_entry_content_range). HTML is discouraged; Markdown is recommended to reduce response size.",
	}, s.getEntryContentFullHandler)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_entry_content_range",
		Description: "Get a content range from an entry (preferred; range required). HTML is discouraged; Markdown is recommended to reduce response size. Use length=0 to return metadata only.",
	}, s.getEntryContentRangeHandler)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "list_categories",
		Description: "Get a list of all categories (summary only)",
	}, s.listCategoriesHandler)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_category_entries",
		Description: "Get entries from a specific category (metadata only, no content)",
	}, s.getCategoryEntriesHandler)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_counters",
		Description: "Get unread and read entry counters",
	}, s.getCountersHandler)

	s.logger.Println("All tools registered successfully")
}

const (
	defaultLimit = 100
	maxLimit     = 1000
)

var (
	allowedStatus    = map[string]struct{}{"read": {}, "unread": {}, "removed": {}}
	allowedOrder     = map[string]struct{}{"id": {}, "status": {}, "published_at": {}, "category_title": {}, "category_id": {}}
	allowedDirection = map[string]struct{}{"asc": {}, "desc": {}}
)

func normalizeEntryFilterOptions(in *miniflux.EntryFilterOptions) (*miniflux.EntryFilterOptions, error) {
	if in == nil {
		in = &miniflux.EntryFilterOptions{}
	}

	limit := in.Limit
	if limit == 0 {
		limit = defaultLimit
	}
	if limit < 0 {
		return nil, fmt.Errorf("invalid limit: %d", in.Limit)
	}
	if limit > maxLimit {
		return nil, fmt.Errorf("limit exceeds maximum (%d): %d", maxLimit, in.Limit)
	}

	offset := in.Offset
	if offset < 0 {
		return nil, fmt.Errorf("invalid offset: %d", in.Offset)
	}

	status := strings.ToLower(strings.TrimSpace(in.Status))
	if status != "" {
		if _, ok := allowedStatus[status]; !ok {
			return nil, fmt.Errorf("invalid status: %s", in.Status)
		}
	}

	order := strings.ToLower(strings.TrimSpace(in.Order))
	if order != "" {
		if _, ok := allowedOrder[order]; !ok {
			return nil, fmt.Errorf("invalid order: %s", in.Order)
		}
	}

	direction := strings.ToLower(strings.TrimSpace(in.Direction))
	if direction != "" {
		if _, ok := allowedDirection[direction]; !ok {
			return nil, fmt.Errorf("invalid direction: %s", in.Direction)
		}
	}

	return &miniflux.EntryFilterOptions{
		Limit:     limit,
		Offset:    offset,
		Status:    status,
		Order:     order,
		Direction: direction,
	}, nil
}

type FeedSummary struct {
	ID            int64  `json:"id"`
	Title         string `json:"title"`
	FeedURL       string `json:"feed_url,omitempty"`
	SiteURL       string `json:"site_url,omitempty"`
	CategoryID    int64  `json:"category_id,omitempty"`
	CategoryTitle string `json:"category_title,omitempty"`
}

type EntrySummary struct {
	ID          int64     `json:"id"`
	FeedID      int64     `json:"feed_id"`
	FeedTitle   string    `json:"feed_title,omitempty"`
	Title       string    `json:"title"`
	URL         string    `json:"url,omitempty"`
	Author      string    `json:"author,omitempty"`
	Status      string    `json:"status,omitempty"`
	PublishedAt time.Time `json:"published_at"`
}

type CategorySummary struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

type FeedCounterSummary struct {
	FeedID int64 `json:"feed_id"`
	Read   int   `json:"read"`
	Unread int   `json:"unread"`
}

func summarizeFeed(feed *minifluxclient.Feed) *FeedSummary {
	if feed == nil {
		return nil
	}

	summary := &FeedSummary{
		ID:      feed.ID,
		Title:   feed.Title,
		FeedURL: feed.FeedURL,
		SiteURL: feed.SiteURL,
	}

	if feed.Category != nil {
		summary.CategoryID = feed.Category.ID
		summary.CategoryTitle = feed.Category.Title
	}

	return summary
}

func summarizeFeeds(feeds minifluxclient.Feeds) []FeedSummary {
	summaries := make([]FeedSummary, 0, len(feeds))
	for _, feed := range feeds {
		if feed == nil {
			continue
		}
		summary := summarizeFeed(feed)
		if summary == nil {
			continue
		}
		summaries = append(summaries, *summary)
	}
	return summaries
}

func summarizeEntry(entry *minifluxclient.Entry) EntrySummary {
	summary := EntrySummary{
		ID:          entry.ID,
		FeedID:      entry.FeedID,
		Title:       entry.Title,
		URL:         entry.URL,
		Author:      entry.Author,
		Status:      entry.Status,
		PublishedAt: entry.Date,
	}

	if entry.Feed != nil {
		if summary.FeedID == 0 {
			summary.FeedID = entry.Feed.ID
		}
		summary.FeedTitle = entry.Feed.Title
	}

	return summary
}

func summarizeEntries(entries minifluxclient.Entries) []EntrySummary {
	summaries := make([]EntrySummary, 0, len(entries))
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		summaries = append(summaries, summarizeEntry(entry))
	}
	return summaries
}

func summarizeCategories(categories minifluxclient.Categories) []CategorySummary {
	summaries := make([]CategorySummary, 0, len(categories))
	for _, category := range categories {
		if category == nil {
			continue
		}
		summaries = append(summaries, CategorySummary{
			ID:    category.ID,
			Title: category.Title,
		})
	}
	return summaries
}

func summarizeCounters(counters *minifluxclient.FeedCounters) []FeedCounterSummary {
	if counters == nil {
		return nil
	}

	byID := make(map[int64]*FeedCounterSummary, len(counters.ReadCounters)+len(counters.UnreadCounters))
	for feedID, count := range counters.ReadCounters {
		byID[feedID] = &FeedCounterSummary{
			FeedID: feedID,
			Read:   count,
		}
	}
	for feedID, count := range counters.UnreadCounters {
		summary, ok := byID[feedID]
		if !ok {
			summary = &FeedCounterSummary{FeedID: feedID}
			byID[feedID] = summary
		}
		summary.Unread = count
	}

	ordered := make([]FeedCounterSummary, 0, len(byID))
	for _, summary := range byID {
		ordered = append(ordered, *summary)
	}
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].FeedID < ordered[j].FeedID
	})

	return ordered
}

type ListFeedsArgs struct {
	Limit  int `json:"limit" jsonschema:"maximum number of feeds to return. Default 100. Max 1000."`
	Offset int `json:"offset" jsonschema:"number of feeds to skip. Must be >= 0. Default 0."`
}

type ListFeedsResult struct {
	Feeds []FeedSummary `json:"feeds"`
}

func (s *Server) listFeedsHandler(ctx context.Context, req *mcp.CallToolRequest, input ListFeedsArgs) (*mcp.CallToolResult, ListFeedsResult, error) {
	feeds, err := s.minifluxClient.GetFeeds()
	if err != nil {
		return nil, ListFeedsResult{}, fmt.Errorf("failed to get feeds: %w", err)
	}

	start := max(input.Offset, 0)
	end := len(feeds)
	if input.Limit > 0 {
		end = min(start+input.Limit, len(feeds))
	}
	if start > len(feeds) {
		start = len(feeds)
	}

	paged := feeds[start:end]

	return nil, ListFeedsResult{Feeds: summarizeFeeds(paged)}, nil
}

type GetFeedArgs struct {
	FeedID int64 `json:"feed_id" jsonschema:"the id of the feed"`
}

type GetFeedResult struct {
	Feed *FeedSummary `json:"feed"`
}

func (s *Server) getFeedHandler(ctx context.Context, req *mcp.CallToolRequest, input GetFeedArgs) (*mcp.CallToolResult, GetFeedResult, error) {
	if input.FeedID == 0 {
		return nil, GetFeedResult{}, fmt.Errorf("feed_id is required")
	}

	feed, err := s.minifluxClient.GetFeed(input.FeedID)
	if err != nil {
		return nil, GetFeedResult{}, fmt.Errorf("failed to get feed: %w", err)
	}

	return nil, GetFeedResult{Feed: summarizeFeed(feed)}, nil
}

type GetFeedEntriesArgs struct {
	FeedID    int64  `json:"feed_id" jsonschema:"the id of the feed"`
	Limit     int    `json:"limit" jsonschema:"maximum number of entries to return. Default 100. Max 1000."`
	Offset    int    `json:"offset" jsonschema:"number of entries to skip. Must be >= 0. Default 0."`
	Status    string `json:"status" jsonschema:"filter by entry status (e.g., unread, read). Invalid values rejected."`
	Order     string `json:"order" jsonschema:"order field (e.g., published_at). Invalid values rejected."`
	Direction string `json:"direction" jsonschema:"sort direction: 'asc' or 'desc' (case-insensitive). Invalid values rejected."`
}

type GetFeedEntriesResult struct {
	Entries []EntrySummary `json:"entries"`
}

func (s *Server) getFeedEntriesHandler(ctx context.Context, req *mcp.CallToolRequest, input GetFeedEntriesArgs) (*mcp.CallToolResult, GetFeedEntriesResult, error) {
	if input.FeedID == 0 {
		return nil, GetFeedEntriesResult{}, fmt.Errorf("feed_id is required")
	}

	raw := &miniflux.EntryFilterOptions{
		Limit:     input.Limit,
		Offset:    input.Offset,
		Status:    input.Status,
		Order:     input.Order,
		Direction: input.Direction,
	}

	opts, err := normalizeEntryFilterOptions(raw)
	if err != nil {
		return nil, GetFeedEntriesResult{}, err
	}

	entries, err := s.minifluxClient.GetFeedEntries(input.FeedID, opts)
	if err != nil {
		return nil, GetFeedEntriesResult{}, fmt.Errorf("failed to get feed entries: %w", err)
	}

	return nil, GetFeedEntriesResult{Entries: summarizeEntries(entries)}, nil
}

type ListEntriesArgs struct {
	Limit     int    `json:"limit" jsonschema:"maximum number of entries to return. Default 100. Max 1000."`
	Offset    int    `json:"offset" jsonschema:"number of entries to skip. Must be >= 0. Default 0."`
	Status    string `json:"status" jsonschema:"filter by entry status (e.g., unread, read). Invalid values rejected."`
	Order     string `json:"order" jsonschema:"order field (e.g., published_at). Invalid values rejected."`
	Direction string `json:"direction" jsonschema:"sort direction: 'asc' or 'desc' (case-insensitive). Invalid values rejected."`
}

type ListEntriesResult struct {
	Entries []EntrySummary `json:"entries"`
}

func (s *Server) listEntriesHandler(ctx context.Context, req *mcp.CallToolRequest, input ListEntriesArgs) (*mcp.CallToolResult, ListEntriesResult, error) {
	raw := &miniflux.EntryFilterOptions{
		Limit:     input.Limit,
		Offset:    input.Offset,
		Status:    input.Status,
		Order:     input.Order,
		Direction: input.Direction,
	}

	opts, err := normalizeEntryFilterOptions(raw)
	if err != nil {
		return nil, ListEntriesResult{}, err
	}

	entries, err := s.minifluxClient.GetEntries(opts)
	if err != nil {
		return nil, ListEntriesResult{}, fmt.Errorf("failed to get entries: %w", err)
	}

	return nil, ListEntriesResult{Entries: summarizeEntries(entries)}, nil
}

type GetEntryArgs struct {
	EntryID int64 `json:"entry_id" jsonschema:"the id of the entry"`
}

type GetEntryResult struct {
	Entry *EntrySummary `json:"entry"`
}

func (s *Server) getEntryHandler(ctx context.Context, req *mcp.CallToolRequest, input GetEntryArgs) (*mcp.CallToolResult, GetEntryResult, error) {
	if input.EntryID == 0 {
		return nil, GetEntryResult{}, fmt.Errorf("entry_id is required")
	}

	entry, err := s.minifluxClient.GetEntry(input.EntryID)
	if err != nil {
		return nil, GetEntryResult{}, fmt.Errorf("failed to get entry: %w", err)
	}

	summary := summarizeEntry(entry)
	return nil, GetEntryResult{Entry: &summary}, nil
}

type GetEntryContentFullArgs struct {
	EntryID int64  `json:"entry_id" jsonschema:"the id of the entry"`
	Format  string `json:"format" jsonschema:"content format (recommended: Markdown; HTML is discouraged). Default: markdown."`
}

type GetEntryContentFullResult struct {
	Content string `json:"content"`
}

func (s *Server) getEntryContentFullHandler(ctx context.Context, req *mcp.CallToolRequest, input GetEntryContentFullArgs) (*mcp.CallToolResult, GetEntryContentFullResult, error) {
	if input.EntryID == 0 {
		return nil, GetEntryContentFullResult{}, fmt.Errorf("entry_id is required")
	}
	if input.Format == "" {
		input.Format = "markdown"
	}

	contentBytes, _, err := s.contentManager.FetchContent(ctx, input.EntryID, content.ContentFormat(input.Format), s.config.Cache.TTL)
	if err != nil {
		return nil, GetEntryContentFullResult{}, fmt.Errorf("failed to fetch content: %w", err)
	}

	return nil, GetEntryContentFullResult{Content: string(contentBytes)}, nil
}

type GetEntryContentRangeArgs struct {
	EntryID   int64  `json:"entry_id" jsonschema:"the id of the entry"`
	Format    string `json:"format" jsonschema:"content format (recommended: Markdown; HTML is discouraged). Default: markdown."`
	Offset    int64  `json:"offset" jsonschema:"required: start offset (character or byte)."`
	Length    int64  `json:"length" jsonschema:"required: length to return (character or byte). Returns up to this length if remaining content is shorter. Use length=0 to return metadata only."`
	RangeType string `json:"range_type" jsonschema:"required: range type (recommended: char; allowed: char, byte)."`
}

type EntryContentRangeMetadata struct {
	TotalLength     int64     `json:"total_length"`
	TotalByteLength int64     `json:"total_byte_length"`
	RangeLength     int64     `json:"range_length"`
	RangeByteLength int64     `json:"range_byte_length"`
	Encoding        string    `json:"encoding,omitempty"`
	Format          string    `json:"format,omitempty"`
	ContentType     string    `json:"content_type,omitempty"`
	FetchedAt       time.Time `json:"fetched_at,omitempty"`
}

type GetEntryContentRangeResult struct {
	Content  string                    `json:"content"`
	Metadata EntryContentRangeMetadata `json:"metadata"`
}

func (s *Server) getEntryContentRangeHandler(ctx context.Context, req *mcp.CallToolRequest, input GetEntryContentRangeArgs) (*mcp.CallToolResult, GetEntryContentRangeResult, error) {
	if input.EntryID == 0 {
		return nil, GetEntryContentRangeResult{}, fmt.Errorf("entry_id is required")
	}
	if input.RangeType == "" {
		return nil, GetEntryContentRangeResult{}, fmt.Errorf("range_type is required")
	}
	if input.Offset < 0 {
		return nil, GetEntryContentRangeResult{}, fmt.Errorf("offset must be >= 0")
	}
	if input.Length < 0 {
		return nil, GetEntryContentRangeResult{}, fmt.Errorf("length must be >= 0")
	}
	if input.Format == "" {
		input.Format = "markdown"
	}

	start := input.Offset
	end := input.Offset + input.Length

	contentBytes, metadata, err := s.contentManager.GetContentRange(ctx, input.EntryID, content.ContentFormat(input.Format), start, end, content.RangeType(input.RangeType), s.config.Cache.TTL)
	if err != nil {
		return nil, GetEntryContentRangeResult{}, fmt.Errorf("failed to get content range: %w", err)
	}

	rangeMetadata := EntryContentRangeMetadata{}
	if metadata != nil {
		rangeMetadata = EntryContentRangeMetadata{
			TotalLength:     metadata.TotalLength,
			TotalByteLength: metadata.TotalByteLength,
			RangeLength:     metadata.Length,
			RangeByteLength: metadata.ByteLength,
			Encoding:        metadata.Encoding,
			Format:          string(metadata.Format),
			ContentType:     metadata.ContentType,
			FetchedAt:       metadata.FetchedAt,
		}
	}

	return nil, GetEntryContentRangeResult{
		Content:  string(contentBytes),
		Metadata: rangeMetadata,
	}, nil
}

type ListCategoriesArgs struct{}

type ListCategoriesResult struct {
	Categories []CategorySummary `json:"categories"`
}

func (s *Server) listCategoriesHandler(ctx context.Context, req *mcp.CallToolRequest, input ListCategoriesArgs) (*mcp.CallToolResult, ListCategoriesResult, error) {
	categories, err := s.minifluxClient.GetCategories()
	if err != nil {
		return nil, ListCategoriesResult{}, fmt.Errorf("failed to get categories: %w", err)
	}

	return nil, ListCategoriesResult{Categories: summarizeCategories(categories)}, nil
}

type GetCategoryEntriesArgs struct {
	CategoryID int64  `json:"category_id" jsonschema:"the id of the category"`
	Limit      int    `json:"limit" jsonschema:"maximum number of entries to return. Default 100. Max 1000."`
	Offset     int    `json:"offset" jsonschema:"number of entries to skip. Must be >= 0. Default 0."`
	Status     string `json:"status" jsonschema:"filter by entry status (e.g., unread, read). Invalid values rejected."`
	Order      string `json:"order" jsonschema:"order field (e.g., published_at). Invalid values rejected."`
	Direction  string `json:"direction" jsonschema:"sort direction: 'asc' or 'desc' (case-insensitive). Invalid values rejected."`
}

type GetCategoryEntriesResult struct {
	Entries []EntrySummary `json:"entries"`
}

func (s *Server) getCategoryEntriesHandler(ctx context.Context, req *mcp.CallToolRequest, input GetCategoryEntriesArgs) (*mcp.CallToolResult, GetCategoryEntriesResult, error) {
	if input.CategoryID == 0 {
		return nil, GetCategoryEntriesResult{}, fmt.Errorf("category_id is required")
	}
	raw := &miniflux.EntryFilterOptions{
		Limit:     input.Limit,
		Offset:    input.Offset,
		Status:    input.Status,
		Order:     input.Order,
		Direction: input.Direction,
	}

	opts, err := normalizeEntryFilterOptions(raw)
	if err != nil {
		return nil, GetCategoryEntriesResult{}, err
	}

	entries, err := s.minifluxClient.GetCategoryEntries(input.CategoryID, opts)
	if err != nil {
		return nil, GetCategoryEntriesResult{}, fmt.Errorf("failed to get category entries: %w", err)
	}

	return nil, GetCategoryEntriesResult{Entries: summarizeEntries(entries)}, nil
}

type GetCountersArgs struct{}

type GetCountersResult struct {
	Counters []FeedCounterSummary `json:"counters"`
}

func (s *Server) getCountersHandler(ctx context.Context, req *mcp.CallToolRequest, input GetCountersArgs) (*mcp.CallToolResult, GetCountersResult, error) {
	counters, err := s.minifluxClient.GetCounters()
	if err != nil {
		return nil, GetCountersResult{}, fmt.Errorf("failed to get counters: %w", err)
	}

	return nil, GetCountersResult{Counters: summarizeCounters(counters)}, nil
}

func (s *Server) Run(ctx context.Context, transport mcp.Transport) error {
	_, err := s.mcpServer.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	<-ctx.Done()
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Println("Shutting down MCP server...")

	if err := s.cacheManager.Clear(); err != nil {
		s.logger.Printf("Failed to clear cache: %v", err)
	}

	return nil
}

func (s *Server) GetMCPServer() *mcp.Server {
	return s.mcpServer
}
