/*
Copyright © 2022 Jonathan Gao <gsmlg.com@gmail.com>

*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	zdns_username string = ""
)

// zdnsCmd represents the zdns command
var zdnsCmd = &cobra.Command{
	Use:   "zdns",
	Short: "Manage zdns cloud zones and record.",
	Long: `Command line tool for zdns domain name server. 
  Login to zdns and manage dns resource records.`,
	Run: func(cmd *cobra.Command, args []string) {
		user, _ := cmd.Flags().GetString("user")

		fmt.Println("zdns")
		if user != "" {
			fmt.Printf("try login as %s\n", user)
		} else if zdns_username != "" {
			fmt.Printf("Already login as %s\n", zdns_username)
		}
	},
}

func init() {
	rootCmd.AddCommand(zdnsCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// zdnsCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// zdnsCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	zdnsCmd.Flags().StringP("user", "u", "", "set zdns_user")

	zdns_username = viper.GetString("zdns_user")
}
