package cli

import (
	"fmt"
	"os"

	"github.com/manifoldco/promptui"
	"github.com/pterm/pterm"
)

type UIConfig struct {
	NoConfirm bool
}

type UI struct {
	config UIConfig
}

func NewUI(noConfirm bool) *UI {
	return &UI{
		config: UIConfig{
			NoConfirm: noConfirm,
		},
	}
}

func (u *UI) Confirm(message string) bool {
	if u.config.NoConfirm {
		return true
	}

	prompt := promptui.Prompt{
		Label:     message,
		IsConfirm: true,
	}

	result, err := prompt.Run()
	if err != nil {
		return false
	}

	return result == "y"
}

func (u *UI) ShowSpinner(message string) *pterm.SpinnerPrinter {
	spinner, _ := pterm.DefaultSpinner.Start(message)
	return spinner
}

func (u *UI) StopSpinner(spinner *pterm.SpinnerPrinter) {
	_ = spinner.Stop()
}

func (u *UI) SelectFromList(label string, items []string) (int, error) {
	if u.config.NoConfirm && len(items) > 0 {
		return 0, nil
	}

	prompt := promptui.Select{
		Label: label,
		Items: items,
	}

	idx, _, err := prompt.Run()
	return idx, err
}

type BatchItem struct {
	ID    int64
	Title string
}

func (u *UI) SelectBatchItems(label string, items []BatchItem) ([]int64, error) {
	if u.config.NoConfirm {
		ids := make([]int64, len(items))
		for i, item := range items {
			ids[i] = item.ID
		}
		return ids, nil
	}

	displayItems := make([]string, len(items))
	selectedMap := make(map[int]bool)

	for i, item := range items {
		displayItems[i] = fmt.Sprintf("   %s", item.Title)
	}

	for {
		prompt := promptui.Select{
			Label: label,
			Items: displayItems,
		}

		idx, _, err := prompt.RunCursorAt(0, 0)
		if err != nil {
			if err.Error() == "^C" {
				return nil, err
			}
			break
		}

		if idx >= 0 && idx < len(displayItems) {
			selectedMap[idx] = !selectedMap[idx]

			if selectedMap[idx] {
				displayItems[idx] = fmt.Sprintf("[✓] %s", items[idx].Title)
			} else {
				displayItems[idx] = fmt.Sprintf("   %s", items[idx].Title)
			}
		}
	}

	selectedIDs := make([]int64, 0)
	for idx, selected := range selectedMap {
		if selected {
			selectedIDs = append(selectedIDs, items[idx].ID)
		}
	}

	return selectedIDs, nil
}

func (u *UI) PrintSuccess(message string) {
	pterm.Success.Println(message)
}

func (u *UI) PrintError(message string) {
	pterm.Error.Println(message)
	os.Exit(1)
}

func (u *UI) PrintInfo(message string) {
	pterm.Info.Println(message)
}
