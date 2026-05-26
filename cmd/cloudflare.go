/*
Copyright © 2026 Jonathan Gao <gsmlg.com@gmail.com>
*/
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/cloudflare/cloudflare-go"
	"github.com/gsmlg-dev/gsmlg-golang/print"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// cloudflareCmd represents the cloudflare command
var cloudflareCmd = &cobra.Command{
	Use:   "cloudflare",
	Short: "Manage Cloudflare zones and DNS records",
	Long:  `Manage Cloudflare zones and DNS records.`,
}

// cloudflareConfigCmd represents the cloudflare config command
var cloudflareConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Configure Cloudflare credentials",
	Run: func(cmd *cobra.Command, args []string) {
		token, _ := cmd.Flags().GetString("token")
		email, _ := cmd.Flags().GetString("email")
		key, _ := cmd.Flags().GetString("key")

		changed := false
		if token != "" {
			viper.Set("cloudflare.token", token)
			changed = true
		}
		if email != "" {
			viper.Set("cloudflare.email", email)
			changed = true
		}
		if key != "" {
			viper.Set("cloudflare.key", key)
			changed = true
		}

		if changed {
			writeConfig()
			fmt.Println("Cloudflare configuration updated successfully.")
		} else {
			t := viper.GetString("cloudflare.token")
			em := viper.GetString("cloudflare.email")
			k := viper.GetString("cloudflare.key")

			maskedToken := ""
			if len(t) > 8 {
				maskedToken = t[:4] + "..." + t[len(t)-4:]
			} else if t != "" {
				maskedToken = "***"
			}

			maskedKey := ""
			if len(k) > 8 {
				maskedKey = k[:4] + "..." + k[len(k)-4:]
			} else if k != "" {
				maskedKey = "***"
			}

			fmt.Printf("Current Cloudflare Token: %s\n", maskedToken)
			fmt.Printf("Current Cloudflare Email: %s\n", em)
			fmt.Printf("Current Cloudflare API Key: %s\n", maskedKey)
		}
	},
}

// cloudflareZoneCmd represents the cloudflare zone command
var cloudflareZoneCmd = &cobra.Command{
	Use:   "zone",
	Short: "Manage Cloudflare zones",
}

type CloudflareZone struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Type   string `json:"type"`
	Paused bool   `json:"paused"`
}

var cloudflareZoneListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all zones",
	Run: func(cmd *cobra.Command, args []string) {
		api, err := getCloudflareClient()
		exitIfError(err)

		zones, err := api.ListZones(context.TODO())
		exitIfError(err)

		cfZones := make([]CloudflareZone, 0)
		for _, zone := range zones {
			cfZones = append(cfZones, CloudflareZone{
				ID:     zone.ID,
				Name:   zone.Name,
				Status: zone.Status,
				Type:   zone.Type,
				Paused: zone.Paused,
			})
		}

		ot, _ := cmd.Flags().GetString("output")
		if ot == "json" {
			print.Json(cfZones)
		} else {
			print.Table(cfZones, []string{"id", "name", "status", "type", "paused"})
		}
	},
}

var cloudflareZoneCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a zone",
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			exitIfError(errors.New("--name is required"))
		}
		zoneType, _ := cmd.Flags().GetString("type")
		accountID, _ := cmd.Flags().GetString("account-id")

		api, err := getCloudflareClient()
		exitIfError(err)

		account := cloudflare.Account{}
		if accountID != "" {
			account.ID = accountID
		}

		zone, err := api.CreateZone(context.TODO(), name, false, account, zoneType)
		exitIfError(err)

		fmt.Printf("Zone created successfully.\nID: %s\nName: %s\nStatus: %s\n", zone.ID, zone.Name, zone.Status)
	},
}

var cloudflareZoneDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a zone",
	Run: func(cmd *cobra.Command, args []string) {
		id, _ := cmd.Flags().GetString("id")
		if id == "" {
			exitIfError(errors.New("--id is required"))
		}

		api, err := getCloudflareClient()
		exitIfError(err)

		res, err := api.DeleteZone(context.TODO(), id)
		exitIfError(err)

		fmt.Printf("Zone deleted successfully.\nID: %s\n", res.ID)
	},
}

// cloudflareRecordCmd represents the cloudflare record command
var cloudflareRecordCmd = &cobra.Command{
	Use:     "record",
	Aliases: []string{"rr"},
	Short:   "Manage DNS records",
}

type CloudflareRecord struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied bool   `json:"proxied"`
}

var cloudflareRecordListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all DNS records in a zone",
	Run: func(cmd *cobra.Command, args []string) {
		zoneID, _ := cmd.Flags().GetString("zone")
		if zoneID == "" {
			exitIfError(errors.New("--zone is required"))
		}

		api, err := getCloudflareClient()
		exitIfError(err)

		records, _, err := api.ListDNSRecords(context.TODO(), cloudflare.ZoneIdentifier(zoneID), cloudflare.ListDNSRecordsParams{})
		exitIfError(err)

		cfRecords := make([]CloudflareRecord, 0)
		for _, rec := range records {
			cfRecords = append(cfRecords, CloudflareRecord{
				ID:      rec.ID,
				Name:    rec.Name,
				Type:    rec.Type,
				Content: rec.Content,
				TTL:     rec.TTL,
				Proxied: *rec.Proxied,
			})
		}

		ot, _ := cmd.Flags().GetString("output")
		if ot == "json" {
			print.Json(cfRecords)
		} else {
			print.Table(cfRecords, []string{"id", "name", "type", "content", "ttl", "proxied"})
		}
	},
}

var cloudflareRecordCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a DNS record",
	Run: func(cmd *cobra.Command, args []string) {
		zoneID, _ := cmd.Flags().GetString("zone")
		name, _ := cmd.Flags().GetString("name")
		rtype, _ := cmd.Flags().GetString("rtype")
		ttl, _ := cmd.Flags().GetInt("ttl")
		rdata, _ := cmd.Flags().GetString("rdata")
		proxied, _ := cmd.Flags().GetBool("proxied")

		if zoneID == "" || name == "" || rtype == "" || rdata == "" {
			exitIfError(errors.New("--zone, --name, --rtype, and --rdata are required"))
		}

		api, err := getCloudflareClient()
		exitIfError(err)

		params := cloudflare.CreateDNSRecordParams{
			Type:    rtype,
			Name:    name,
			Content: rdata,
			TTL:     ttl,
			Proxied: &proxied,
		}

		rec, err := api.CreateDNSRecord(context.TODO(), cloudflare.ZoneIdentifier(zoneID), params)
		exitIfError(err)

		fmt.Printf("DNS record created successfully.\nID: %s\nName: %s\nType: %s\nContent: %s\n", rec.ID, rec.Name, rec.Type, rec.Content)
	},
}

var cloudflareRecordDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a DNS record",
	Run: func(cmd *cobra.Command, args []string) {
		zoneID, _ := cmd.Flags().GetString("zone")
		id, _ := cmd.Flags().GetString("id")

		if zoneID == "" || id == "" {
			exitIfError(errors.New("--zone and --id are required"))
		}

		api, err := getCloudflareClient()
		exitIfError(err)

		err = api.DeleteDNSRecord(context.TODO(), cloudflare.ZoneIdentifier(zoneID), id)
		exitIfError(err)

		fmt.Println("DNS record deleted successfully.")
	},
}

func getCloudflareClient() (*cloudflare.API, error) {
	token := viper.GetString("cloudflare.token")
	email := viper.GetString("cloudflare.email")
	key := viper.GetString("cloudflare.key")

	if token != "" {
		return cloudflare.NewWithAPIToken(token)
	}
	if email != "" && key != "" {
		return cloudflare.New(key, email)
	}

	envToken := os.Getenv("CLOUDFLARE_API_TOKEN")
	if envToken != "" {
		return cloudflare.NewWithAPIToken(envToken)
	}
	envEmail := os.Getenv("CLOUDFLARE_API_EMAIL")
	envKey := os.Getenv("CLOUDFLARE_API_KEY")
	if envEmail != "" && envKey != "" {
		return cloudflare.New(envKey, envEmail)
	}

	return nil, errors.New("missing Cloudflare credentials. Configure with 'gsmlg-cli cloudflare config' or set CLOUDFLARE_API_TOKEN environment variable")
}

func init() {
	rootCmd.AddCommand(cloudflareCmd)

	// Config Command
	cloudflareCmd.AddCommand(cloudflareConfigCmd)
	cloudflareConfigCmd.Flags().String("token", "", "Cloudflare API Token")
	cloudflareConfigCmd.Flags().String("email", "", "Cloudflare Email")
	cloudflareConfigCmd.Flags().String("key", "", "Cloudflare API Key")

	// Zone Commands
	cloudflareCmd.AddCommand(cloudflareZoneCmd)
	cloudflareZoneCmd.PersistentFlags().StringP("output", "o", "", "Print format, json or plain text")

	cloudflareZoneCmd.AddCommand(cloudflareZoneListCmd)

	cloudflareZoneCmd.AddCommand(cloudflareZoneCreateCmd)
	cloudflareZoneCreateCmd.Flags().StringP("name", "n", "", "Zone name (e.g. example.com)")
	cloudflareZoneCreateCmd.Flags().StringP("type", "t", "full", "Zone type (full or partial)")
	cloudflareZoneCreateCmd.Flags().String("account-id", "", "Account ID")

	cloudflareZoneCmd.AddCommand(cloudflareZoneDeleteCmd)
	cloudflareZoneDeleteCmd.Flags().StringP("id", "i", "", "Zone ID")

	// Record Commands
	cloudflareCmd.AddCommand(cloudflareRecordCmd)
	cloudflareRecordCmd.PersistentFlags().StringP("output", "o", "", "Print format, json or plain text")
	cloudflareRecordCmd.PersistentFlags().StringP("zone", "z", "", "Zone ID")

	cloudflareRecordCmd.AddCommand(cloudflareRecordListCmd)

	cloudflareRecordCmd.AddCommand(cloudflareRecordCreateCmd)
	cloudflareRecordCreateCmd.Flags().StringP("name", "n", "", "Record name")
	cloudflareRecordCreateCmd.Flags().StringP("rtype", "t", "", "Record type (A, AAAA, CNAME, TXT, MX, SRV, etc.)")
	cloudflareRecordCreateCmd.Flags().Int("ttl", 1, "Record TTL in seconds (1 for automatic)")
	cloudflareRecordCreateCmd.Flags().StringP("rdata", "d", "", "Record content/data")
	cloudflareRecordCreateCmd.Flags().Bool("proxied", false, "Proxy through Cloudflare network")

	cloudflareRecordCmd.AddCommand(cloudflareRecordDeleteCmd)
	cloudflareRecordDeleteCmd.Flags().StringP("id", "i", "", "Record ID")
}
