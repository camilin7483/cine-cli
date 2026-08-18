package cli

import (
	"fmt"

	"github.com/cam/cine-cli/internal/plugin"
	"github.com/spf13/cobra"
)

func (a *App) pluginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage plugins",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List installed plugins",
			RunE: func(cmd *cobra.Command, args []string) error {
				plugins := a.Plugins.List()
				if len(plugins) == 0 {
					fmt.Println("No plugins installed.")
					return nil
				}
				if a.jsonOut {
					var manifests []interface{}
					for _, p := range plugins {
						manifests = append(manifests, p.Manifest)
					}
					a.printJSON(manifests)
					return nil
				}
				for _, p := range plugins {
					status := "enabled"
					if !p.Manifest.Enabled {
						status = "disabled"
					}
					fmt.Printf("  %s v%s [%s] by %s — %s\n",
						p.Manifest.Name, p.Manifest.Version, status, p.Manifest.Author, p.Manifest.Description)
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "enable <name>",
			Short: "Enable a plugin",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return a.Plugins.Enable(args[0])
			},
		},
		&cobra.Command{
			Use:   "disable <name>",
			Short: "Disable a plugin",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return a.Plugins.Disable(args[0])
			},
		},
		&cobra.Command{
			Use:   "discover",
			Short: "Discover plugins in plugin directory",
			RunE: func(cmd *cobra.Command, args []string) error {
				return a.Plugins.Discover(cmd.Context())
			},
		},
		&cobra.Command{
			Use:   "doc",
			Short: "Show plugin development documentation",
			RunE: func(cmd *cobra.Command, args []string) error {
				fmt.Print(plugin.PluginDoc)
				return nil
			},
		},
	)
	return cmd
}
