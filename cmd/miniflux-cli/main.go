package main

import (
	"log"

	"github.com/itoooak/miniflux-utils/internal/cli"
	"github.com/itoooak/miniflux-utils/internal/miniflux"
	"github.com/itoooak/miniflux-utils/pkg/utils"
	"github.com/spf13/cobra"
)

var (
	noConfirm bool
	cfg       *utils.Config
	client    *miniflux.Client
	ui        *cli.UI
)

var rootCmd = &cobra.Command{
	Use:   "miniflux-cli",
	Short: "Miniflux CLI for managing feeds, categories, and entries",
	Long:  "CLI tool for viewing and managing Miniflux feeds, categories, and entries",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		cfg, err = utils.LoadConfig()
		if err != nil {
			log.Fatalf("failed to load config: %v", err)
		}

		client, err = miniflux.NewClient(&cfg.Miniflux)
		if err != nil {
			log.Fatalf("failed to create miniflux client: %v", err)
		}

		ui = cli.NewUI(noConfirm)
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&noConfirm, "no-confirm", false, "skip confirmation prompts")

	feedCmd := &cobra.Command{
		Use:   "feed",
		Short: "Manage feeds",
	}

	createFeedCmd := &cobra.Command{
		Use:   "create <url>",
		Short: "Create a new feed",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			categoryID, _ := cmd.Flags().GetString("category")
			return feedCreateCmd(args[0], categoryID)
		},
	}
	createFeedCmd.Flags().String("category", "", "category ID")
	feedCmd.AddCommand(createFeedCmd)

	feedCmd.AddCommand(
		&cobra.Command{
			Use:   "delete IDS...",
			Short: "Delete feeds with confirmation",
			Args:  cobra.MinimumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return feedDeleteCmd(args)
			},
		},
	)

	rootCmd.AddCommand(feedCmd)

	categoryCmd := &cobra.Command{
		Use:   "category",
		Short: "Manage categories",
	}

	categoryCmd.AddCommand(
		&cobra.Command{
			Use:   "create <title>",
			Short: "Create a new category",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return categoryCreateCmd(args[0])
			},
		},
		&cobra.Command{
			Use:   "update <category-id> <title>",
			Short: "Update a category",
			Args:  cobra.ExactArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				return categoryUpdateCmd(args[0], args[1])
			},
		},
		&cobra.Command{
			Use:   "delete IDS...",
			Short: "Delete categories with confirmation",
			Args:  cobra.MinimumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return categoryDeleteCmd(args)
			},
		},
	)

	rootCmd.AddCommand(categoryCmd)

	entryCmd := &cobra.Command{
		Use:   "entry",
		Short: "Manage entries",
	}

	entryCmd.AddCommand(
		&cobra.Command{
			Use:   "mark-read IDS...",
			Short: "Mark entries as read",
			Args:  cobra.MinimumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return entryMarkReadCmd(args)
			},
		},
		&cobra.Command{
			Use:   "mark-unread IDS...",
			Short: "Mark entries as unread",
			Args:  cobra.MinimumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return entryMarkUnreadCmd(args)
			},
		},
	)

	rootCmd.AddCommand(entryCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
