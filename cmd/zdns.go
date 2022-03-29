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
			// fmt.Printf("User: %s, Password: %s\n", user, pass)
			u := zdns.Login(user, pass)
			fmt.Printf("zdns user: %v\n", u.Username)
			viper.Set("zdnsuser", u)

			writeConfig()
		} else if zdns_username != "" {
			fmt.Printf("Already login as %s\n", zdns_username)
			token := viper.GetString("zdnsuser.token")
			zdns.SetToken(token)
			zdns.GetZone()
		} else {
			fmt.Printf("Your haven't login yet. Please use --username [your name] to login.\n")
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
	zdnsCmd.Flags().StringP("username", "u", "", "login username")
	zdnsCmd.Flags().StringP("password", "p", "", "login password")

}
