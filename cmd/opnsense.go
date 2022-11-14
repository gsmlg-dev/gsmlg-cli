/*
Copyright © 2022 Jonathan Gao <gsmlg.com@gmail.com>
*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// opnsenseCmd represents the opnsense command
var opnsenseCmd = &cobra.Command{
	Use:   "opnsense",
	Short: "Manage opnsense, start/stop/reconfigure services.",
	Long: `Command line tool for opnsense router management.
  Login to opnsense and manage services.`,
	Run: func(cmd *cobra.Command, args []string) {
		opnsense_token := viper.GetString("opnsense.token")
		server_url := viper.GetString("opnsense.server_url")

		token, err := cmd.Flags().GetString("token")
		exitIfError(err)
		url, err := cmd.Flags().GetString("url")
		exitIfError(err)

		if url != "" || token != "" {
			if url != "" {
				viper.Set("opnsense.url", url)
			}
			if token != "" {
				viper.Set("opnsense.token", token)
			}
			writeConfig()
		} else if opnsense_token != "" || server_url != "" {
			fmt.Printf("Token is setted as %s\n", opnsense_token)
			fmt.Printf("Server URL is: %s", server_url)
		} else {
			fmt.Printf("Your don't have token yet. Please use --token [your token] to login.\n")
		}
	},
}

func init() {
	rootCmd.AddCommand(opnsenseCmd)

	opnsenseCmd.Flags().StringP("token", "t", "", "opnsene user token")
	opnsenseCmd.Flags().StringP("url", "l", "", "opnsene user token")
}
