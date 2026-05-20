/*
Copyright © 2022 Jonathan Gao <gsmlg.com@gmail.com>
*/
package cmd

import (
	"errors"
	"os"

	"github.com/gsmlg-dev/gsmlg-golang/print"
	"github.com/gsmlg-dev/gsmlg-golang/zdns"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// zoneGroupCmd represents the zoneGroup command
var zoneGroupCmd = &cobra.Command{
	Use:   "zone-group",
	Short: "List ZDNS zone groups",
	Long:  `List all zone groups from the ZDNS cloud service. Requires prior authentication via 'zdns login'.`,
	Run: func(cmd *cobra.Command, args []string) {
		uname := viper.GetString("zdnsuser.username")
		if uname == "" {
			exitIfError(errors.New("need login first"))
		}
		token := viper.GetString("zdnsuser.token")
		zdns.SetToken(token)
		zgs, err := zdns.GetZoneGroup()
		ot, _ := cmd.Flags().GetString("output")
		if err != nil {
			print.Error(err)
			os.Exit(1)
		} else if ot == "json" {
			print.Json(zgs)
		} else {
			print.Table(zgs, []string{"id", "name", "create_time"})
		}
	},
}

func init() {
	zdnsCmd.AddCommand(zoneGroupCmd)
}
