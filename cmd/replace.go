/*
Copyright © 2022 Jonathan Gao <gsmlg.com@gmail.com>
*/
package cmd

import (
	"github.com/spf13/cobra"
)

// replaceCmd replace strings
var replaceCmd = &cobra.Command{
	Use:   "replace",
	Short: "replace string in directory.",
	Long: `replace string in directory.
  gsmlg-cli replace --from "<name1>" --to "<name2>" --only-in <dir?>.`,
	Run: func(cmd *cobra.Command, args []string) {
		from, err := cmd.Flags().GetString("from")
		exitIfError(err)
		to, err := cmd.Flags().GetString("to")
		exitIfError(err)
		onlyIn, err := cmd.Flags().GetString("only-in")
		exitIfError(err)

		if from != "" || to != "" {
			if onlyIn != "" {
				// replace in only in dir

			} else {
				// replace in cwd
			}
		} else if to == "" || from != "" {
			// replace to is empty, do search only
			if onlyIn != "" {
				// search in only in dir
			} else {
				// search in cwd
			}
		} else {
			// print help
			cmd.Help()
		}
	},
}

func init() {
	rootCmd.AddCommand(replaceCmd)

	replaceCmd.Flags().StringP("from", "f", "", "replace from")
	replaceCmd.Flags().StringP("to", "t", "", "replace to")
	replaceCmd.Flags().StringP("only-in", "d", "", "replace only in directory")
}
