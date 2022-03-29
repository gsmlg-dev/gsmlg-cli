/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"errors"

	"github.com/gsmlg-dev/gsmlg-golang/print"
	"github.com/gsmlg-dev/gsmlg-golang/zdns"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// zoneGroupCmd represents the zoneGroup command
var zoneGroupCmd = &cobra.Command{
	Use:   "zone-group",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		uname := viper.GetString("zdnsuser.username")
		if uname == "" {
			exitIfError(errors.New("Need login first."))
		}
		token := viper.GetString("zdnsuser.token")
		zdns.SetToken(token)
		zgs := zdns.GetZoneGroup()
		ot, _ := cmd.Flags().GetString("output")
		if ot == "json" {
			print.Json(zgs)
		} else {
			print.Table(zgs, []string{"id", "name", "create_time"})
		}
	},
}

func init() {
	zdnsCmd.AddCommand(zoneGroupCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// zoneGroupCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// zoneGroupCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
