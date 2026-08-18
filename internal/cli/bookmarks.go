package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

func (a *App) bookmarksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bookmarks",
		Short: "Manage scene bookmarks",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List bookmarks",
		RunE: func(cmd *cobra.Command, args []string) error {
			items, err := a.DB.ListBookmarks(cmd.Context(), 50)
			if err != nil {
				return err
			}
			if len(items) == 0 {
				fmt.Println("No bookmarks yet.")
				return nil
			}
			for _, b := range items {
				ep := ""
				if b.Season > 0 {
					ep = fmt.Sprintf(" S%02dE%02d", b.Season, b.Episode)
				}
				fmt.Printf("  #%d  %s%s  @ %.0fs  %s\n", b.ID, b.Title, ep, b.Position, b.Note)
			}
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "add <media_id> <title> [position_seconds]",
		Short: "Add a bookmark",
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			pos := 0.0
			if len(args) > 2 {
				pos, _ = strconv.ParseFloat(args[2], 64)
			}
			return a.DB.AddBookmark(cmd.Context(), args[0], args[1], "movie", 0, 0, pos, "")
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "delete <id>",
		Short: "Delete bookmark by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, _ := strconv.ParseInt(args[0], 10, 64)
			return a.DB.DeleteBookmark(cmd.Context(), id)
		},
	})
	return cmd
}
