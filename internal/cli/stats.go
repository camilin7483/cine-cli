package cli

import (
	"github.com/cam/cine-cli/internal/stats"
	"github.com/spf13/cobra"
)

func (a *App) statsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show viewing statistics dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			d := stats.NewDashboard(a.DB, a.Config)
			if err := d.Collect(cmd.Context()); err != nil {
				return err
			}
			if a.jsonOut {
				a.printJSON(d)
				return nil
			}
			cmd.Print(d.Format())
			return nil
		},
	}
	cmd.Flags().Bool("json", false, "JSON output")
	return cmd
}
