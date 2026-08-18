package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cam/cine-cli/internal/core"
	"github.com/spf13/cobra"
)

func (a *App) offlineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "offline",
		Short: "List and play local downloads (offline mode)",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List downloaded files",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := a.Config.DownloadDirPath()
			entries, err := os.ReadDir(dir)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Println("No downloads directory yet.")
					return nil
				}
				return err
			}
			n := 0
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				name := e.Name()
				low := strings.ToLower(name)
				if !strings.HasSuffix(low, ".mp4") && !strings.HasSuffix(low, ".mkv") && !strings.HasSuffix(low, ".webm") && !strings.HasSuffix(low, ".m3u8") {
					continue
				}
				info, _ := e.Info()
				size := int64(0)
				if info != nil {
					size = info.Size()
				}
				fmt.Printf("  %s  (%.1f MB)\n", name, float64(size)/(1024*1024))
				n++
			}
			if n == 0 {
				fmt.Println("No local media files found in", dir)
			} else {
				fmt.Printf("\n%d file(s) in %s\n", n, dir)
			}
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "play <filename>",
		Short: "Play a local downloaded file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := a.Config.DownloadDirPath()
			path := args[0]
			if !filepath.IsAbs(path) {
				path = filepath.Join(dir, path)
			}
			if _, err := os.Stat(path); err != nil {
				return fmt.Errorf("file not found: %s", path)
			}
			opts := a.playLocalOpts(path)
			return a.Player.Play(cmd.Context(), opts)
		},
	})
	return cmd
}

func (a *App) playLocalOpts(path string) core.PlayOptions {
	return core.PlayOptions{
		StreamURL: path,
		Title:     filepath.Base(path),
		Player:    a.Config.Player,
	}
}
