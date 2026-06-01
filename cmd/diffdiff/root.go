package main

import "github.com/spf13/cobra"

var cfgFile string

// newRootCmd builds the command tree. The root command is the actual diffdiff:
// it takes no arguments and launches the desktop viewer on the current
// directory's repository (use the in-app picker to open another). The config and
// version utility subcommands hang off it.
func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "diffdiff",
		Short:         "diffdiff renders the current git repository's working-tree diff",
		Long:          "A fast, themeable desktop viewer for the current git repository's working-tree diff.",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runGUI(cmd.Context())
		},
	}

	cmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path")
	cmd.AddCommand(newConfigCmd())
	cmd.AddCommand(newVersionCmd())

	return cmd
}
