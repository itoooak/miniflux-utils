package miniflux

import (
	"context"
	"fmt"
	"strconv"

	"github.com/itoooak/miniflux-utils/pkg/utils"
	minifluxclient "miniflux.app/v2/client"
)

type Client struct {
	client *minifluxclient.Client
	config *utils.MinifluxConfig
}

func NewClient(cfg *utils.MinifluxConfig) (*Client, error) {
	if cfg.APIToken == "" {
		return nil, fmt.Errorf("API token is required")
	}

	client := minifluxclient.NewClientWithOptions(
		cfg.URL,
		minifluxclient.WithAPIKey(cfg.APIToken),
	)

	return &Client{
		client: client,
		config: cfg,
	}, nil
}

func (c *Client) GetFeeds() (minifluxclient.Feeds, error) {
	feeds, err := c.client.Feeds()
	if err != nil {
		return nil, fmt.Errorf("failed to get feeds: %w", err)
	}
	return feeds, nil
}

func (c *Client) GetFeed(feedID int64) (*minifluxclient.Feed, error) {
	feed, err := c.client.Feed(feedID)
	if err != nil {
		return nil, fmt.Errorf("failed to get feed %d: %w", feedID, err)
	}
	return feed, nil
}

func (c *Client) GetFeedEntries(feedID int64, opts *EntryFilterOptions) (minifluxclient.Entries, error) {
	if opts == nil {
		opts = &EntryFilterOptions{}
	}

	filters := &minifluxclient.Filter{
		Limit:     opts.Limit,
		Offset:    opts.Offset,
		Status:    opts.Status,
		Order:     opts.Order,
		Direction: opts.Direction,
	}

	result, err := c.client.FeedEntries(feedID, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to get feed %d entries: %w", feedID, err)
	}
	return result.Entries, nil
}

func (c *Client) GetEntries(opts *EntryFilterOptions) (minifluxclient.Entries, error) {
	if opts == nil {
		opts = &EntryFilterOptions{}
	}

	filters := &minifluxclient.Filter{
		Limit:     opts.Limit,
		Offset:    opts.Offset,
		Status:    opts.Status,
		Order:     opts.Order,
		Direction: opts.Direction,
	}

	result, err := c.client.Entries(filters)
	if err != nil {
		return nil, fmt.Errorf("failed to get entries: %w", err)
	}
	return result.Entries, nil
}

func (c *Client) GetEntry(entryID int64) (*minifluxclient.Entry, error) {
	entry, err := c.client.Entry(entryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get entry %d: %w", entryID, err)
	}
	return entry, nil
}

func (c *Client) GetCategories() (minifluxclient.Categories, error) {
	categories, err := c.client.Categories()
	if err != nil {
		return nil, fmt.Errorf("failed to get categories: %w", err)
	}
	return categories, nil
}

func (c *Client) GetCategoryEntries(categoryID int64, opts *EntryFilterOptions) (minifluxclient.Entries, error) {
	if opts == nil {
		opts = &EntryFilterOptions{}
	}

	filters := &minifluxclient.Filter{
		Limit:     opts.Limit,
		Offset:    opts.Offset,
		Status:    opts.Status,
		Order:     opts.Order,
		Direction: opts.Direction,
	}

	result, err := c.client.CategoryEntries(categoryID, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to get category %d entries: %w", categoryID, err)
	}
	return result.Entries, nil
}

func (c *Client) GetCurrentUser() (*minifluxclient.User, error) {
	user, err := c.client.Me()
	if err != nil {
		return nil, fmt.Errorf("failed to get current user: %w", err)
	}
	return user, nil
}

func (c *Client) GetCounters() (*minifluxclient.FeedCounters, error) {
	counters, err := c.client.FetchCounters()
	if err != nil {
		return nil, fmt.Errorf("failed to get counters: %w", err)
	}
	return counters, nil
}

func (c *Client) FetchOriginalContent(ctx context.Context, entryID int64) (string, error) {
	content, err := c.client.FetchEntryOriginalContentContext(ctx, entryID)
	if err != nil {
		return "", fmt.Errorf("failed to fetch original content for entry %d: %w", entryID, err)
	}
	return content, nil
}

func (c *Client) CreateFeed(url, categoryID string) (*minifluxclient.Feed, error) {
	req := &minifluxclient.FeedCreationRequest{
		FeedURL: url,
	}
	if categoryID != "" {
		catID, err := strconv.ParseInt(categoryID, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid category ID: %w", err)
		}
		req.CategoryID = catID
	}

	feedID, err := c.client.CreateFeed(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create feed: %w", err)
	}

	feed, err := c.client.Feed(feedID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created feed: %w", err)
	}
	return feed, nil
}

func (c *Client) UpdateFeed(feedID int64, feedChanges *minifluxclient.FeedModificationRequest) error {
	_, err := c.client.UpdateFeed(feedID, feedChanges)
	if err != nil {
		return fmt.Errorf("failed to update feed %d: %w", feedID, err)
	}
	return nil
}

func (c *Client) DeleteFeed(feedID int64) error {
	err := c.client.DeleteFeed(feedID)
	if err != nil {
		return fmt.Errorf("failed to delete feed %d: %w", feedID, err)
	}
	return nil
}

func (c *Client) CreateCategory(title string) (*minifluxclient.Category, error) {
	category, err := c.client.CreateCategory(title)
	if err != nil {
		return nil, fmt.Errorf("failed to create category: %w", err)
	}
	return category, nil
}

func (c *Client) UpdateCategory(categoryID int64, title string) error {
	_, err := c.client.UpdateCategory(categoryID, title)
	if err != nil {
		return fmt.Errorf("failed to update category %d: %w", categoryID, err)
	}
	return nil
}

func (c *Client) DeleteCategory(categoryID int64) error {
	err := c.client.DeleteCategory(categoryID)
	if err != nil {
		return fmt.Errorf("failed to delete category %d: %w", categoryID, err)
	}
	return nil
}

func (c *Client) UpdateEntry(entryID int64, status string) error {
	err := c.client.UpdateEntries([]int64{entryID}, status)
	if err != nil {
		return fmt.Errorf("failed to update entry %d: %w", entryID, err)
	}
	return nil
}

type EntryFilterOptions struct {
	Limit     int
	Offset    int
	Status    string
	Order     string
	Direction string
}
