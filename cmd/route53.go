/*
Copyright © 2026 Jonathan Gao <gsmlg.com@gmail.com>
*/
package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/gsmlg-dev/gsmlg-golang/print"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// route53Cmd represents the route53 command
var route53Cmd = &cobra.Command{
	Use:   "route53",
	Short: "Manage AWS Route53 hosted zones and records",
	Long:  `Manage AWS Route53 hosted zones and records.`,
}

// route53ConfigCmd represents the route53 config command
var route53ConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Configure AWS credentials for Route53",
	Run: func(cmd *cobra.Command, args []string) {
		accessKey, _ := cmd.Flags().GetString("access-key-id")
		secretKey, _ := cmd.Flags().GetString("secret-access-key")
		region, _ := cmd.Flags().GetString("region")

		changed := false
		if accessKey != "" {
			viper.Set("aws.access_key_id", accessKey)
			changed = true
		}
		if secretKey != "" {
			viper.Set("aws.secret_access_key", secretKey)
			changed = true
		}
		if region != "" {
			viper.Set("aws.region", region)
			changed = true
		}

		if changed {
			writeConfig()
			fmt.Println("AWS Route53 configuration updated successfully.")
		} else {
			ak := viper.GetString("aws.access_key_id")
			reg := viper.GetString("aws.region")
			maskedAk := ""
			if len(ak) > 8 {
				maskedAk = ak[:4] + "..." + ak[len(ak)-4:]
			} else if ak != "" {
				maskedAk = "***"
			}
			fmt.Printf("Current AWS Access Key ID: %s\n", maskedAk)
			fmt.Printf("Current AWS Region: %s\n", reg)
		}
	},
}

// route53ZoneCmd represents the route53 zone command
var route53ZoneCmd = &cobra.Command{
	Use:   "zone",
	Short: "Manage hosted zones",
}

type Route53Zone struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	RecordCount int64  `json:"record_count"`
	PrivateZone bool   `json:"private_zone"`
	Comment     string `json:"comment"`
}

var route53ZoneListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all hosted zones",
	Run: func(cmd *cobra.Command, args []string) {
		client, err := getRoute53Client()
		exitIfError(err)

		input := &route53.ListHostedZonesInput{}
		output, err := client.ListHostedZones(context.TODO(), input)
		exitIfError(err)

		zones := make([]Route53Zone, 0)
		for _, hz := range output.HostedZones {
			comment := ""
			if hz.Config != nil && hz.Config.Comment != nil {
				comment = *hz.Config.Comment
			}
			recordCount := int64(0)
			if hz.ResourceRecordSetCount != nil {
				recordCount = *hz.ResourceRecordSetCount
			}
			id := aws.ToString(hz.Id)
			id = strings.TrimPrefix(id, "/hostedzone/")

			zones = append(zones, Route53Zone{
				ID:          id,
				Name:        aws.ToString(hz.Name),
				RecordCount: recordCount,
				PrivateZone: hz.Config != nil && hz.Config.PrivateZone,
				Comment:     comment,
			})
		}

		ot, _ := cmd.Flags().GetString("output")
		if ot == "json" {
			print.Json(zones)
		} else {
			print.Table(zones, []string{"id", "name", "record_count", "private_zone", "comment"})
		}
	},
}

var route53ZoneCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a hosted zone",
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			exitIfError(errors.New("--name is required"))
		}
		comment, _ := cmd.Flags().GetString("comment")
		private, _ := cmd.Flags().GetBool("private")

		client, err := getRoute53Client()
		exitIfError(err)

		input := &route53.CreateHostedZoneInput{
			Name:            aws.String(name),
			CallerReference: aws.String(fmt.Sprintf("%d", time.Now().UnixNano())),
		}
		if comment != "" || private {
			input.HostedZoneConfig = &types.HostedZoneConfig{
				Comment:     aws.String(comment),
				PrivateZone: private,
			}
		}

		output, err := client.CreateHostedZone(context.TODO(), input)
		exitIfError(err)

		fmt.Printf("Hosted zone created successfully.\nID: %s\nName: %s\n",
			strings.TrimPrefix(aws.ToString(output.HostedZone.Id), "/hostedzone/"),
			aws.ToString(output.HostedZone.Name))
	},
}

var route53ZoneDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a hosted zone",
	Run: func(cmd *cobra.Command, args []string) {
		id, _ := cmd.Flags().GetString("id")
		if id == "" {
			exitIfError(errors.New("--id is required"))
		}

		client, err := getRoute53Client()
		exitIfError(err)

		zoneID := id
		if !strings.HasPrefix(zoneID, "/hostedzone/") {
			zoneID = "/hostedzone/" + zoneID
		}

		input := &route53.DeleteHostedZoneInput{
			Id: aws.String(zoneID),
		}
		_, err = client.DeleteHostedZone(context.TODO(), input)
		exitIfError(err)

		fmt.Println("Hosted zone deleted successfully.")
	},
}

// route53RecordCmd represents the route53 record command
var route53RecordCmd = &cobra.Command{
	Use:     "record",
	Aliases: []string{"rr"},
	Short:   "Manage resource record sets",
}

type Route53Record struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	TTL   int64  `json:"ttl"`
	Value string `json:"value"`
}

var route53RecordListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all record sets in a hosted zone",
	Run: func(cmd *cobra.Command, args []string) {
		zoneID, _ := cmd.Flags().GetString("zone")
		if zoneID == "" {
			exitIfError(errors.New("--zone is required"))
		}

		client, err := getRoute53Client()
		exitIfError(err)

		if !strings.HasPrefix(zoneID, "/hostedzone/") {
			zoneID = "/hostedzone/" + zoneID
		}

		input := &route53.ListResourceRecordSetsInput{
			HostedZoneId: aws.String(zoneID),
		}
		output, err := client.ListResourceRecordSets(context.TODO(), input)
		exitIfError(err)

		records := make([]Route53Record, 0)
		for _, rrs := range output.ResourceRecordSets {
			ttl := int64(0)
			if rrs.TTL != nil {
				ttl = *rrs.TTL
			}
			var values []string
			for _, val := range rrs.ResourceRecords {
				values = append(values, aws.ToString(val.Value))
			}
			valueStr := strings.Join(values, ", ")
			if rrs.AliasTarget != nil {
				valueStr = fmt.Sprintf("ALIAS %s", aws.ToString(rrs.AliasTarget.DNSName))
			}

			records = append(records, Route53Record{
				Name:  aws.ToString(rrs.Name),
				Type:  string(rrs.Type),
				TTL:   ttl,
				Value: valueStr,
			})
		}

		ot, _ := cmd.Flags().GetString("output")
		if ot == "json" {
			print.Json(records)
		} else {
			print.Table(records, []string{"name", "type", "ttl", "value"})
		}
	},
}

var route53RecordCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create or update a record set",
	Run: func(cmd *cobra.Command, args []string) {
		zoneID, _ := cmd.Flags().GetString("zone")
		name, _ := cmd.Flags().GetString("name")
		rtype, _ := cmd.Flags().GetString("rtype")
		ttl, _ := cmd.Flags().GetInt64("ttl")
		rdata, _ := cmd.Flags().GetString("rdata")

		if zoneID == "" || name == "" || rtype == "" || rdata == "" {
			exitIfError(errors.New("--zone, --name, --rtype, and --rdata are required"))
		}

		client, err := getRoute53Client()
		exitIfError(err)

		if !strings.HasPrefix(zoneID, "/hostedzone/") {
			zoneID = "/hostedzone/" + zoneID
		}

		var rrs []types.ResourceRecord
		for _, val := range strings.Split(rdata, ",") {
			val = strings.TrimSpace(val)
			if val != "" {
				rrs = append(rrs, types.ResourceRecord{
					Value: aws.String(val),
				})
			}
		}

		input := &route53.ChangeResourceRecordSetsInput{
			HostedZoneId: aws.String(zoneID),
			ChangeBatch: &types.ChangeBatch{
				Changes: []types.Change{
					{
						Action: types.ChangeActionUpsert,
						ResourceRecordSet: &types.ResourceRecordSet{
							Name:            aws.String(name),
							Type:            types.RRType(strings.ToUpper(rtype)),
							TTL:             aws.Int64(ttl),
							ResourceRecords: rrs,
						},
					},
				},
			},
		}

		_, err = client.ChangeResourceRecordSets(context.TODO(), input)
		exitIfError(err)

		fmt.Println("Record set created/updated successfully.")
	},
}

var route53RecordDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a record set",
	Run: func(cmd *cobra.Command, args []string) {
		zoneID, _ := cmd.Flags().GetString("zone")
		name, _ := cmd.Flags().GetString("name")
		rtype, _ := cmd.Flags().GetString("rtype")
		ttl, _ := cmd.Flags().GetInt64("ttl")
		rdata, _ := cmd.Flags().GetString("rdata")

		if zoneID == "" || name == "" || rtype == "" || rdata == "" {
			exitIfError(errors.New("--zone, --name, --rtype, and --rdata are required"))
		}

		client, err := getRoute53Client()
		exitIfError(err)

		if !strings.HasPrefix(zoneID, "/hostedzone/") {
			zoneID = "/hostedzone/" + zoneID
		}

		var rrs []types.ResourceRecord
		for _, val := range strings.Split(rdata, ",") {
			val = strings.TrimSpace(val)
			if val != "" {
				rrs = append(rrs, types.ResourceRecord{
					Value: aws.String(val),
				})
			}
		}

		input := &route53.ChangeResourceRecordSetsInput{
			HostedZoneId: aws.String(zoneID),
			ChangeBatch: &types.ChangeBatch{
				Changes: []types.Change{
					{
						Action: types.ChangeActionDelete,
						ResourceRecordSet: &types.ResourceRecordSet{
							Name:            aws.String(name),
							Type:            types.RRType(strings.ToUpper(rtype)),
							TTL:             aws.Int64(ttl),
							ResourceRecords: rrs,
						},
					},
				},
			},
		}

		_, err = client.ChangeResourceRecordSets(context.TODO(), input)
		exitIfError(err)

		fmt.Println("Record set deleted successfully.")
	},
}

func getRoute53Client() (*route53.Client, error) {
	ctx := context.TODO()
	accessKey := viper.GetString("aws.access_key_id")
	secretKey := viper.GetString("aws.secret_access_key")
	region := viper.GetString("aws.region")

	if region == "" {
		region = "us-east-1" // default region for Route53 API endpoints
	}

	var cfg aws.Config
	var err error

	if accessKey != "" && secretKey != "" {
		cfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(region),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
		)
	} else {
		cfg, err = config.LoadDefaultConfig(ctx, config.WithRegion(region))
	}
	if err != nil {
		return nil, err
	}

	return route53.NewFromConfig(cfg), nil
}

func init() {
	rootCmd.AddCommand(route53Cmd)

	// Config Command
	route53Cmd.AddCommand(route53ConfigCmd)
	route53ConfigCmd.Flags().String("access-key-id", "", "AWS Access Key ID")
	route53ConfigCmd.Flags().String("secret-access-key", "", "AWS Secret Access Key")
	route53ConfigCmd.Flags().String("region", "", "AWS Region")

	// Zone Commands
	route53Cmd.AddCommand(route53ZoneCmd)
	route53ZoneCmd.PersistentFlags().StringP("output", "o", "", "Print format, json or plain text")

	route53ZoneCmd.AddCommand(route53ZoneListCmd)

	route53ZoneCmd.AddCommand(route53ZoneCreateCmd)
	route53ZoneCreateCmd.Flags().StringP("name", "n", "", "Zone name (e.g. example.com.)")
	route53ZoneCreateCmd.Flags().StringP("comment", "c", "", "Zone comment")
	route53ZoneCreateCmd.Flags().Bool("private", false, "Create a private hosted zone")

	route53ZoneCmd.AddCommand(route53ZoneDeleteCmd)
	route53ZoneDeleteCmd.Flags().StringP("id", "i", "", "Zone ID")

	// Record Commands
	route53Cmd.AddCommand(route53RecordCmd)
	route53RecordCmd.PersistentFlags().StringP("output", "o", "", "Print format, json or plain text")
	route53RecordCmd.PersistentFlags().StringP("zone", "z", "", "Hosted Zone ID")

	route53RecordCmd.AddCommand(route53RecordListCmd)

	route53RecordCmd.AddCommand(route53RecordCreateCmd)
	route53RecordCreateCmd.Flags().StringP("name", "n", "", "Record name")
	route53RecordCreateCmd.Flags().StringP("rtype", "t", "", "Record type (A, AAAA, CNAME, TXT, MX, etc.)")
	route53RecordCreateCmd.Flags().Int64("ttl", 300, "Record TTL in seconds")
	route53RecordCreateCmd.Flags().StringP("rdata", "d", "", "Record data (values, comma-separated)")

	route53RecordCmd.AddCommand(route53RecordDeleteCmd)
	route53RecordDeleteCmd.Flags().StringP("name", "n", "", "Record name")
	route53RecordDeleteCmd.Flags().StringP("rtype", "t", "", "Record type")
	route53RecordDeleteCmd.Flags().Int64("ttl", 300, "Record TTL in seconds")
	route53RecordDeleteCmd.Flags().StringP("rdata", "d", "", "Record data")
}
