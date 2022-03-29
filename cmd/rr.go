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

// rrCmd represents the rr command
var rrCmd = &cobra.Command{
	Use:   "rr",
	Short: "Zdns Resouce Record management",
	Long:  `Manage zdns rrs.`,
	Run: func(cmd *cobra.Command, args []string) {
		uname := viper.GetString("zdnsuser.username")
		if uname == "" {
			exitIfError(errors.New("Need login first."))
		}
		token := viper.GetString("zdnsuser.token")
		zdns.SetToken(token)
		ot, _ := cmd.Flags().GetString("output")
		zone, _ := cmd.Flags().GetString("zone")
		isCreate, _ := cmd.Flags().GetBool("create")
		isUpdate, _ := cmd.Flags().GetBool("update")
		isDelete, _ := cmd.Flags().GetBool("delete")
		if isCreate {
			zone, _ := cmd.Flags().GetString("zone")
			name, _ := cmd.Flags().GetString("name")
			ttl, _ := cmd.Flags().GetInt("ttl")
			rtype, _ := cmd.Flags().GetString("rtype")
			rdata, _ := cmd.Flags().GetString("rdata")
			rrs := zdns.CreateRrInZone(zone, name, rtype, ttl, rdata)
			if ot == "json" {
				print.Json(rrs)
			} else {
				print.Table(rrs, []string{"id", "name", "ttl", "type", "rdata", "view", "note", "flags"})
			}
		} else if isUpdate {

		} else if isDelete {
			id, _ := cmd.Flags().GetString("id")
			rrs := zdns.DeleteRr(id)
			if ot == "json" {
				print.Json(rrs)
			} else {
				print.Table(rrs, []string{"id", "name", "ttl", "type", "rdata", "view", "note", "flags"})
			}
		} else {
			if zone == "" {
				exitIfError(errors.New("--zone [zone] is required"))
			}
			rrs := zdns.GetRrByZone(zone)

			if ot == "json" {
				print.Json(rrs)
			} else {
				print.Table(rrs, []string{"id", "name", "ttl", "type", "rdata", "view", "note", "flags"})
			}
		}
	},
}

func init() {
	zdnsCmd.AddCommand(rrCmd)

	rrCmd.PersistentFlags().StringP("output", "o", "", "Print format, json or plain text")

	rrCmd.Flags().BoolP("create", "c", false, "Create rr")
	rrCmd.Flags().BoolP("update", "u", false, "Update rr")
	rrCmd.Flags().BoolP("delete", "d", false, "Delete rr")

	rrCmd.Flags().String("zone", "", "Zone id")
	// err := rrCmd.MarkFlagRequired("zone")
	// exitIfError(err)
	rrCmd.Flags().String("id", "", "Rr id")

	rrCmd.Flags().String("name", "@", "Rr name")
	rrCmd.Flags().Int("ttl", 3600, "Rr ttl")
	rrCmd.Flags().String("rtype", "a", "Rr type")
	rrCmd.Flags().String("rdata", "", "Rr data")

}
