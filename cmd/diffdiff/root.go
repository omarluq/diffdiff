package main

import "github.com/spf13/cobra"

var cfgFile string

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "diffdiff [path]",
		Short:         "diffdiff renders git diffs from a local repository",
		Long:          "A fast, themeable desktop viewer for a local git repository's working-tree diff.",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			repoPath := "."
			if len(args) == 1 {
				repoPath = args[0]
			}

			return runGUI(cmd.Context(), repoPath)
		},
	}

	cmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path")
	cmd.AddCommand(newConfigCmd())
	cmd.AddCommand(newVersionCmd())

	return cmd
}
