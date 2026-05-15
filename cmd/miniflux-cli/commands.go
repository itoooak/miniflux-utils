package main

import (
	"fmt"
	"strconv"

	"github.com/itoooak/miniflux-utils/internal/cli"
)

func parseIDs(args []string, label string) ([]int64, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("at least one %s ID required", label)
	}

	ids := make([]int64, len(args))
	for i, arg := range args {
		id, err := strconv.ParseInt(arg, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid %s ID '%s': %v", label, arg, err)
		}
		ids[i] = id
	}

	return ids, nil
}

func feedCreateCmd(url, categoryID string) error {
	spinner := ui.ShowSpinner("Creating feed...")

	feed, err := client.CreateFeed(url, categoryID)
	ui.StopSpinner(spinner)

	if err != nil {
		ui.PrintError(fmt.Sprintf("Failed to create feed: %v", err))
		return err
	}

	ui.PrintSuccess(fmt.Sprintf("Created feed: %s (ID: %d)", feed.Title, feed.ID))
	return nil
}

func feedDeleteCmd(feedIDStrs []string) error {
	ids, err := parseIDs(feedIDStrs, "feed")
	if err != nil {
		ui.PrintError(err.Error())
		return err
	}

	confirmed := make([]int64, 0, len(ids))
	for _, feedID := range ids {
		feed, err := client.GetFeed(feedID)
		if err != nil {
			ui.PrintInfo(fmt.Sprintf("Feed %d not found", feedID))
			continue
		}

		if ui.Confirm(fmt.Sprintf("Delete feed '%s' (ID: %d)?", feed.Title, feed.ID)) {
			confirmed = append(confirmed, feed.ID)
		}
	}

	if len(confirmed) == 0 {
		ui.PrintInfo("No feeds confirmed for deletion")
		return nil
	}

	spinner := ui.ShowSpinner("Deleting feeds...")

	for _, feedID := range confirmed {
		if err := client.DeleteFeed(feedID); err != nil {
			ui.StopSpinner(spinner)
			ui.PrintError(fmt.Sprintf("Failed to delete feed %d: %v", feedID, err))
			return err
		}
	}
	ui.StopSpinner(spinner)

	ui.PrintSuccess(fmt.Sprintf("Deleted %d feed(s)", len(confirmed)))
	return nil
}

func categoryCreateCmd(title string) error {
	spinner := ui.ShowSpinner("Creating category...")

	category, err := client.CreateCategory(title)
	ui.StopSpinner(spinner)

	if err != nil {
		ui.PrintError(fmt.Sprintf("Failed to create category: %v", err))
		return err
	}

	ui.PrintSuccess(fmt.Sprintf("Created category: %s (ID: %d)", category.Title, category.ID))
	return nil
}

func categoryUpdateCmd(categoryIDStr, title string) error {
	categoryID, err := strconv.ParseInt(categoryIDStr, 10, 64)
	if err != nil {
		ui.PrintError("Invalid category ID")
		return err
	}

	if !ui.Confirm(fmt.Sprintf("Update category to '%s'?", title)) {
		ui.PrintInfo("Cancelled")
		return nil
	}

	spinner := ui.ShowSpinner("Updating category...")

	if err := client.UpdateCategory(categoryID, title); err != nil {
		ui.StopSpinner(spinner)
		ui.PrintError(fmt.Sprintf("Failed to update category: %v", err))
		return err
	}
	ui.StopSpinner(spinner)

	ui.PrintSuccess(fmt.Sprintf("Updated category: %s", title))
	return nil
}

func categoryDeleteCmd(categoryIDStrs []string) error {
	ids, err := parseIDs(categoryIDStrs, "category")
	if err != nil {
		ui.PrintError(err.Error())
		return err
	}

	categories, err := client.GetCategories()
	if err != nil {
		ui.PrintError(fmt.Sprintf("Failed to list categories: %v", err))
		return err
	}

	categoryTitles := make(map[int64]string, len(categories))
	for _, cat := range categories {
		categoryTitles[cat.ID] = cat.Title
	}

	confirmed := make([]int64, 0, len(ids))
	for _, categoryID := range ids {
		title, ok := categoryTitles[categoryID]
		if !ok {
			ui.PrintInfo(fmt.Sprintf("Category %d not found", categoryID))
			continue
		}
		if ui.Confirm(fmt.Sprintf("Delete category '%s' (ID: %d)?", title, categoryID)) {
			confirmed = append(confirmed, categoryID)
		}
	}

	if len(confirmed) == 0 {
		ui.PrintInfo("No categories confirmed for deletion")
		return nil
	}

	spinner := ui.ShowSpinner("Deleting categories...")

	for _, categoryID := range confirmed {
		if err := client.DeleteCategory(categoryID); err != nil {
			ui.StopSpinner(spinner)
			ui.PrintError(fmt.Sprintf("Failed to delete category %d: %v", categoryID, err))
			return err
		}
	}
	ui.StopSpinner(spinner)

	ui.PrintSuccess(fmt.Sprintf("Deleted %d categor(ies)", len(confirmed)))
	return nil
}

func entryMarkReadCmd(entryIDStrs []string) error {
	return markEntries(entryIDStrs, "read")
}

func entryMarkUnreadCmd(entryIDStrs []string) error {
	return markEntries(entryIDStrs, "unread")
}

func markEntries(entryIDStrs []string, status string) error {
	ids, err := parseIDs(entryIDStrs, "entry")
	if err != nil {
		ui.PrintError(err.Error())
		return err
	}

	items := make([]cli.BatchItem, 0, len(ids))
	for _, id := range ids {
		entry, err := client.GetEntry(id)
		if err != nil {
			ui.PrintInfo(fmt.Sprintf("Entry %d not found", id))
			continue
		}
		items = append(items, cli.BatchItem{
			ID:    entry.ID,
			Title: entry.Title,
		})
	}

	if len(items) == 0 {
		ui.PrintInfo(fmt.Sprintf("No valid entries found for marking as %s", status))
		return nil
	}

	confirmedIDs := make([]int64, 0, len(items))
	for _, item := range items {
		if ui.Confirm(fmt.Sprintf("Mark entry '%s' (ID: %d) as %s?", item.Title, item.ID, status)) {
			confirmedIDs = append(confirmedIDs, item.ID)
		}
	}

	if len(confirmedIDs) == 0 {
		ui.PrintInfo(fmt.Sprintf("No entries confirmed for marking as %s", status))
		return nil
	}

	spinner := ui.ShowSpinner(fmt.Sprintf("Marking entries as %s...", status))

	for _, id := range confirmedIDs {
		if err := client.UpdateEntry(id, status); err != nil {
			ui.StopSpinner(spinner)
			ui.PrintError(fmt.Sprintf("Failed to mark entry %d as %s: %v", id, status, err))
			return err
		}
	}
	ui.StopSpinner(spinner)

	ui.PrintSuccess(fmt.Sprintf("Marked %d entries as %s", len(confirmedIDs), status))
	return nil
}
