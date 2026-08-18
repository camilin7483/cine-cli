package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cam/cine-cli/internal/core"
	"github.com/spf13/cobra"
)

func (a *App) watchlistCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watchlist",
		Short: "Manage watchlist",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "list [status]",
			Short: "List watchlist (status: pending, watching, completed)",
			Args:  cobra.MaximumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				var items []core.WatchlistItem
				var err error
				if len(args) > 0 {
					items, err = a.DB.ListWatchlistWithStatus(cmd.Context(), args[0])
				} else {
					items, err = a.DB.ListWatchlist(cmd.Context())
				}
				if err != nil {
					return err
				}
				if len(items) == 0 {
					fmt.Println("Watchlist is empty.")
					return nil
				}
				if a.jsonOut {
					a.printJSON(items)
					return nil
				}
				for _, item := range items {
					label := fmt.Sprintf("%s [%s]", item.Title, item.Status)
					if item.MediaType == core.MediaTypeSeries && item.Season > 0 {
						label = fmt.Sprintf("%s S%02dE%02d [%s]", item.Title, item.Season, item.Episode, item.Status)
					}
					fmt.Printf("  %s\n", label)
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "add <media_id> <title>",
			Short: "Add to watchlist",
			Args:  cobra.MinimumNArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				return a.DB.AddWatchlistItem(cmd.Context(), core.WatchlistItem{
					MediaID: args[0],
					Title:   strings.Join(args[1:], " "),
					Status:  "pending",
					AddedAt: time.Now(),
				})
			},
		},
		&cobra.Command{
			Use:   "remove <media_id>",
			Short: "Remove from watchlist",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return a.DB.RemoveWatchlistItem(cmd.Context(), args[0])
			},
		},
		&cobra.Command{
			Use:   "update <media_id> <status>",
			Short: "Update watchlist status",
			Args:  cobra.ExactArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				return a.DB.UpdateWatchlistStatus(cmd.Context(), args[0], args[1])
			},
		},
		&cobra.Command{
			Use:   "export [file]",
			Short: "Export watchlist to JSON",
			Args:  cobra.MaximumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				data, err := a.DB.ExportWatchlistJSON(cmd.Context())
				if err != nil {
					return err
				}
				if len(args) > 0 {
					return os.WriteFile(args[0], data, 0644)
				}
				fmt.Println(string(data))
				return nil
			},
		},
		&cobra.Command{
			Use:   "import <file>",
			Short: "Import watchlist from JSON",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				data, err := os.ReadFile(args[0])
				if err != nil {
					return err
				}
				count, err := a.DB.ImportWatchlistJSON(cmd.Context(), data)
				if err != nil {
					return err
				}
				fmt.Printf("Imported %d items.\n", count)
				return nil
			},
		},
		&cobra.Command{
			Use:   "backup",
			Short: "Backup watchlist to file",
			RunE: func(cmd *cobra.Command, args []string) error {
				backupDir := filepath.Join(a.Config.DataDir, "backups")
				_ = os.MkdirAll(backupDir, 0755)
				path := filepath.Join(backupDir, fmt.Sprintf("watchlist-%s.json", time.Now().Format("20060102-150405")))
				if err := a.DB.BackupWatchlist(cmd.Context(), path); err != nil {
					return err
				}
				fmt.Printf("Backup saved to %s\n", path)
				return nil
			},
		},
		&cobra.Command{
			Use:   "restore <file>",
			Short: "Restore watchlist from backup",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return a.DB.RestoreWatchlist(cmd.Context(), args[0])
			},
		},
		&cobra.Command{
			Use:   "dedup",
			Short: "Remove duplicate watchlist items",
			RunE: func(cmd *cobra.Command, args []string) error {
				count, err := a.DB.RemoveDuplicateWatchlistItems(cmd.Context())
				if err != nil {
					return err
				}
				fmt.Printf("Removed %d duplicates.\n", count)
				return nil
			},
		},
	)
	return cmd
}
