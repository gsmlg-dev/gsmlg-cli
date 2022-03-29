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

// zoneCmd represents the zone command
var zoneCmd = &cobra.Command{
	Use:   "zone",
	Short: "Manage zdns zones",
	Long: `Mange zdns zones, list, create, delete and update.
Use --create to create with --name zone name, zone name must end with "."
`,
	Run: func(cmd *cobra.Command, args []string) {
		uname := viper.GetString("zdnsuser.username")
		if uname == "" {
			exitIfError(errors.New("Need login first."))
		}
		token := viper.GetString("zdnsuser.token")
		zdns.SetToken(token)
		ot, _ := cmd.Flags().GetString("output")
		isCreate, _ := cmd.Flags().GetBool("create")
		isUpdate, _ := cmd.Flags().GetBool("update")
		isDelete, _ := cmd.Flags().GetBool("delete")
		if isCreate {
			zoneName, _ := cmd.Flags().GetString("name")
			zones := zdns.CreateZone(zoneName)
			if ot == "json" {
				print.Json(zones)
			} else {
				print.Table(zones, []string{"id", "name", "create_time", "note", "flags"})
			}
		} else if isUpdate {

		} else if isDelete {
			id, _ := cmd.Flags().GetString("id")
			zones := zdns.DeleteZone(id)
			if ot == "json" {
				print.Json(zones)
			} else {
				print.Table(zones, []string{"id", "name", "create_time", "note", "flags"})
			}
		} else {

			zones := zdns.GetZone()

			if ot == "json" {
				print.Json(zones)
			} else {
				print.Table(zones, []string{"id", "name", "create_time", "note", "flags"})
			}
		}
	},
}

func init() {
	zdnsCmd.AddCommand(zoneCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	zoneCmd.PersistentFlags().StringP("output", "o", "", "Print format, json or plain text")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	zoneCmd.Flags().BoolP("create", "c", false, "Create zone")
	zoneCmd.Flags().BoolP("update", "u", false, "Update zone")
	zoneCmd.Flags().BoolP("delete", "d", false, "Delete zone")

	zoneCmd.Flags().String("name", "", "Zone name")
	zoneCmd.Flags().String("id", "", "Zone id")
}
