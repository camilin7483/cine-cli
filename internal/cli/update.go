package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/cam/cine-cli/internal/update"
	"github.com/spf13/cobra"
)

func (a *App) updateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Check for updates and update cine",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runUpdate(cmd.Context())
		},
	}
}

func (a *App) runUpdate(ctx context.Context) error {
	fmt.Printf("Current version: %s\n", a.Updater.CurrentVersion())
	fmt.Print("Checking for updates... ")

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	release, hasUpdate, err := a.Updater.Check(ctx)
	if err != nil {
		fmt.Println("failed")
		return fmt.Errorf("check update: %w", err)
	}

	if !hasUpdate || release == nil {
		fmt.Println("up to date!")
		fmt.Printf("You have the latest version: %s\n", a.Updater.CurrentVersion())
		return nil
	}

	fmt.Printf("new version available: %s\n", release.TagName)
	fmt.Printf("Release notes: %s\n", release.Body)
	fmt.Print("Downloading... ")

	dlPath := update.DefaultDownloadPath()
	if err := a.Updater.Download(ctx, release, dlPath); err != nil {
		fmt.Println("failed")
		return fmt.Errorf("download: %w", err)
	}

	fmt.Println("done!")
	fmt.Print("Replacing binary... ")

	if err := a.Updater.ReplaceBinary(dlPath); err != nil {
		return fmt.Errorf("replace binary: %w", err)
	}

	fmt.Println("done!")
	fmt.Print("Restarting... ")

	return a.Updater.Restart()
}
