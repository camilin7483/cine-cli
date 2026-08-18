package cli

import (
	"fmt"

	"github.com/cam/cine-cli/internal/core"
	"github.com/spf13/cobra"
)

func (a *App) downloadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "download",
		Short: "Manage downloads",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "list [status]",
			Short: "List downloads (status: queued, downloading, paused, completed, failed, cancelled)",
			Args:  cobra.MaximumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				var status core.DownloadStatus
				if len(args) > 0 {
					status = core.DownloadStatus(args[0])
				}
				dls, err := a.Downloads.List(cmd.Context(), status)
				if err != nil {
					return err
				}
				if len(dls) == 0 {
					fmt.Println("No downloads.")
					return nil
				}
				if a.jsonOut {
					a.printJSON(dls)
					return nil
				}
				for _, dl := range dls {
					prog := fmt.Sprintf("%.1f%%", dl.Progress)
					fmt.Printf("  [%s] %s — %s (%s)\n", dl.Status, dl.Title, prog, dl.Quality)
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "pause <id>",
			Short: "Pause a download",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return a.Downloads.Pause(cmd.Context(), args[0])
			},
		},
		&cobra.Command{
			Use:   "resume <id>",
			Short: "Resume a download",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return a.Downloads.Resume(cmd.Context(), args[0])
			},
		},
		&cobra.Command{
			Use:   "cancel <id>",
			Short: "Cancel a download",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return a.Downloads.Cancel(cmd.Context(), args[0])
			},
		},
		&cobra.Command{
			Use:   "progress <id>",
			Short: "Show download progress",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				dl, err := a.Downloads.Get(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				if dl == nil {
					return fmt.Errorf("download %s not found", args[0])
				}
				if a.jsonOut {
					a.printJSON(dl)
					return nil
				}
				fmt.Printf("Title:    %s\n", dl.Title)
				fmt.Printf("Status:   %s\n", dl.Status)
				fmt.Printf("Progress: %.1f%%\n", dl.Progress)
				if dl.TotalBytes > 0 {
					fmt.Printf("Size:     %s / %s\n", humanBytes(dl.Downloaded), humanBytes(dl.TotalBytes))
				}
				if dl.Speed > 0 {
					fmt.Printf("Speed:    %s/s\n", humanBytes(int64(dl.Speed)))
				}
				fmt.Printf("File:     %s\n", dl.FilePath)
				return nil
			},
		},
		&cobra.Command{
			Use:   "cleanup",
			Short: "Remove old completed downloads",
			RunE: func(cmd *cobra.Command, args []string) error {
				return a.Downloads.Cleanup(cmd.Context())
			},
		},
	)
	return cmd
}

func humanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

