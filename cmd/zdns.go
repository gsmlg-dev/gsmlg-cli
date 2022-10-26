/*
Copyright © 2022 Jonathan Gao <gsmlg.com@gmail.com>
*/
package cmd

import (
	"fmt"
	"syscall"

	"github.com/gsmlg-dev/gsmlg-golang/zdns"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

// zdnsCmd represents the zdns command
var zdnsCmd = &cobra.Command{
	Use:   "zdns",
	Short: "Manage zdns cloud zones and record.",
	Long: `Command line tool for zdns domain name server.
  Login to zdns and manage dns resource records.`,
	Run: func(cmd *cobra.Command, args []string) {
		zdns_username := viper.GetString("zdnsuser.username")

		user, err := cmd.Flags().GetString("username")
		exitIfError(err)

		if user != "" {
			pass, err := cmd.Flags().GetString("password")
			exitIfError(err)
			if pass == "" {
				fmt.Printf("try login as %s\n", user)
				fmt.Print("Enter Password: ")
				bytePassword, err := term.ReadPassword(int(syscall.Stdin))
				exitIfError(err)
				pass = string(bytePassword)
			}
			hours, _ := cmd.Flags().GetInt("hours")
			// fmt.Printf("User: %s, Password: %s\n", user, pass)
			u, err := zdns.Login(user, pass, hours)
			exitIfError(err)
			fmt.Printf("zdns user: %v\n", u.Username)
			viper.Set("zdnsuser", u)

			writeConfig()
		} else if zdns_username != "" {
			fmt.Printf("Already login as %s\n", zdns_username)
		} else {
			fmt.Printf("Your haven't login yet. Please use --username [your name] to login.\n")
		}
	},
}

func init() {
	rootCmd.AddCommand(zdnsCmd)

	zdnsCmd.Flags().StringP("username", "u", "", "login username")
	zdnsCmd.Flags().StringP("password", "p", "", "login password")
	zdnsCmd.Flags().Int("hours", 24*7, "token valid hours")
}
