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

func (a *App) favoritesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "favorites",
		Short: "Manage favorites",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List favorites",
			RunE: func(cmd *cobra.Command, args []string) error {
				favs, err := a.DB.ListFavorites(cmd.Context())
				if err != nil {
					return err
				}
				if len(favs) == 0 {
					fmt.Println("No favorites yet.")
					return nil
				}
				if a.jsonOut {
					a.printJSON(favs)
					return nil
				}
				for _, f := range favs {
					fmt.Printf("  %s (%s) [%s]\n", f.Title, f.MediaType, f.AddedAt.Format("2006-01-02"))
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "add <media_id> <title>",
			Short: "Add to favorites",
			Args:  cobra.MinimumNArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				return a.DB.AddFavorite(cmd.Context(), core.Favorite{
					MediaID: args[0],
					Title:   strings.Join(args[1:], " "),
					AddedAt: time.Now(),
				})
			},
		},
		&cobra.Command{
			Use:   "remove <media_id>",
			Short: "Remove from favorites",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return a.DB.RemoveFavorite(cmd.Context(), args[0])
			},
		},
		&cobra.Command{
			Use:   "export [file]",
			Short: "Export favorites to JSON",
			Args:  cobra.MaximumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				data, err := a.DB.ExportFavoritesJSON(cmd.Context())
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
			Short: "Import favorites from JSON",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				data, err := os.ReadFile(args[0])
				if err != nil {
					return err
				}
				count, err := a.DB.ImportFavoritesJSON(cmd.Context(), data)
				if err != nil {
					return err
				}
				fmt.Printf("Imported %d favorites.\n", count)
				return nil
			},
		},
		&cobra.Command{
			Use:   "backup",
			Short: "Backup favorites to file",
			RunE: func(cmd *cobra.Command, args []string) error {
				backupDir := filepath.Join(a.Config.DataDir, "backups")
				_ = os.MkdirAll(backupDir, 0755)
				path := filepath.Join(backupDir, fmt.Sprintf("favorites-%s.json", time.Now().Format("20060102-150405")))
				if err := a.DB.BackupFavorites(cmd.Context(), path); err != nil {
					return err
				}
				fmt.Printf("Backup saved to %s\n", path)
				return nil
			},
		},
		&cobra.Command{
			Use:   "restore <file>",
			Short: "Restore favorites from backup",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return a.DB.RestoreFavorites(cmd.Context(), args[0])
			},
		},
		&cobra.Command{
			Use:   "dedup",
			Short: "Remove duplicate favorites",
			RunE: func(cmd *cobra.Command, args []string) error {
				count, err := a.DB.RemoveDuplicateFavorites(cmd.Context())
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
