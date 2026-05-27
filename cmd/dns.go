/*
Copyright © 2026 Jonathan Gao <gsmlg.com@gmail.com>
*/
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/cloudflare/cloudflare-go"
	"github.com/gsmlg-dev/gsmlg-golang/print"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// ProviderConfig represents configuration for a DNS provider
type ProviderConfig struct {
	Name            string `mapstructure:"name"`
	Type            string `mapstructure:"type"`
	Token           string `mapstructure:"token,omitempty"`
	Email           string `mapstructure:"email,omitempty"`
	Key             string `mapstructure:"key,omitempty"`
	AccessKeyID     string `mapstructure:"access_key_id,omitempty"`
	SecretAccessKey string `mapstructure:"secret_access_key,omitempty"`
	Region          string `mapstructure:"region,omitempty"`
}

// DnsConfig holds list of configured DNS providers
type DnsConfig struct {
	Providers []ProviderConfig `mapstructure:"providers"`
}

// DnsZone represents a generic DNS zone
type DnsZone struct {
	Provider    string `json:"provider"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	RecordCount string `json:"record_count"`
}

// DnsRecord represents a generic DNS resource record
type DnsRecord struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	TTL     int    `json:"ttl"`
	Value   string `json:"content"`
	Proxied string `json:"proxied"`
}

// DnsProvider interface abstracts Cloudflare and Route53 DNS operations
type DnsProvider interface {
	Name() string
	Type() string
	ListZones(ctx context.Context) ([]DnsZone, error)
	DeleteZone(ctx context.Context, id string) error
	ListRecords(ctx context.Context, zoneID string) ([]DnsRecord, error)
	AddRecord(ctx context.Context, zoneID string, record DnsRecord) error
	ReplaceRecord(ctx context.Context, zoneID string, record DnsRecord) error
	DeleteRecord(ctx context.Context, zoneID string, record DnsRecord) error
}

// CloudflareProvider implements DnsProvider
type CloudflareProvider struct {
	name  string
	email string
	token string
	key   string
	api   *cloudflare.API
}

func (c *CloudflareProvider) Name() string { return c.name }
func (c *CloudflareProvider) Type() string { return "cloudflare" }

func (c *CloudflareProvider) ListZones(ctx context.Context) ([]DnsZone, error) {
	zones, err := c.api.ListZones(ctx)
	if err != nil {
		return nil, err
	}
	var res []DnsZone
	for _, z := range zones {
		res = append(res, DnsZone{
			Provider:    c.name,
			ID:          z.ID,
			Name:        z.Name,
			Status:      z.Status,
			RecordCount: "-",
		})
	}
	return res, nil
}

func (c *CloudflareProvider) DeleteZone(ctx context.Context, id string) error {
	res, err := c.api.DeleteZone(ctx, id)
	if err != nil {
		return err
	}
	fmt.Printf("Deleted Zone ID: %s\n", res.ID)
	return nil
}

func (c *CloudflareProvider) ListRecords(ctx context.Context, zoneID string) ([]DnsRecord, error) {
	records, _, err := c.api.ListDNSRecords(ctx, cloudflare.ZoneIdentifier(zoneID), cloudflare.ListDNSRecordsParams{})
	if err != nil {
		return nil, err
	}
	var res []DnsRecord
	for _, r := range records {
		proxied := "false"
		if r.Proxied != nil {
			proxied = fmt.Sprintf("%t", *r.Proxied)
		}
		res = append(res, DnsRecord{
			ID:      r.ID,
			Name:    r.Name,
			Type:    r.Type,
			TTL:     r.TTL,
			Value:   r.Content,
			Proxied: proxied,
		})
	}
	return res, nil
}

func (c *CloudflareProvider) AddRecord(ctx context.Context, zoneID string, record DnsRecord) error {
	proxied := record.Proxied == "true"
	params := cloudflare.CreateDNSRecordParams{
		Type:    record.Type,
		Name:    record.Name,
		Content: record.Value,
		TTL:     record.TTL,
		Proxied: &proxied,
	}
	_, err := c.api.CreateDNSRecord(ctx, cloudflare.ZoneIdentifier(zoneID), params)
	return err
}

func (c *CloudflareProvider) ReplaceRecord(ctx context.Context, zoneID string, record DnsRecord) error {
	id := record.ID
	if id == "" {
		recs, _, err := c.api.ListDNSRecords(ctx, cloudflare.ZoneIdentifier(zoneID), cloudflare.ListDNSRecordsParams{
			Name: record.Name,
			Type: record.Type,
		})
		if err != nil {
			return err
		}
		if len(recs) == 0 {
			return c.AddRecord(ctx, zoneID, record)
		}
		id = recs[0].ID
	}

	proxied := record.Proxied == "true"
	params := cloudflare.UpdateDNSRecordParams{
		ID:      id,
		Type:    record.Type,
		Name:    record.Name,
		Content: record.Value,
		TTL:     record.TTL,
		Proxied: &proxied,
	}
	_, err := c.api.UpdateDNSRecord(ctx, cloudflare.ZoneIdentifier(zoneID), params)
	return err
}

func (c *CloudflareProvider) DeleteRecord(ctx context.Context, zoneID string, record DnsRecord) error {
	id := record.ID
	if id == "" {
		recs, _, err := c.api.ListDNSRecords(ctx, cloudflare.ZoneIdentifier(zoneID), cloudflare.ListDNSRecordsParams{
			Name: record.Name,
			Type: record.Type,
		})
		if err != nil {
			return err
		}
		for _, r := range recs {
			if record.Value == "" || r.Content == record.Value {
				err = c.api.DeleteDNSRecord(ctx, cloudflare.ZoneIdentifier(zoneID), r.ID)
				if err != nil {
					return err
				}
			}
		}
		return nil
	}
	return c.api.DeleteDNSRecord(ctx, cloudflare.ZoneIdentifier(zoneID), id)
}

// Route53Provider implements DnsProvider
type Route53Provider struct {
	name   string
	client *route53.Client
}

func (r *Route53Provider) Name() string { return r.name }
func (r *Route53Provider) Type() string { return "route53" }

func (r *Route53Provider) ListZones(ctx context.Context) ([]DnsZone, error) {
	input := &route53.ListHostedZonesInput{}
	output, err := r.client.ListHostedZones(ctx, input)
	if err != nil {
		return nil, err
	}
	var res []DnsZone
	for _, hz := range output.HostedZones {
		id := aws.ToString(hz.Id)
		id = strings.TrimPrefix(id, "/hostedzone/")
		recordCount := int64(0)
		if hz.ResourceRecordSetCount != nil {
			recordCount = *hz.ResourceRecordSetCount
		}
		status := "public"
		if hz.Config != nil && hz.Config.PrivateZone {
			status = "private"
		}
		res = append(res, DnsZone{
			Provider:    r.name,
			ID:          id,
			Name:        aws.ToString(hz.Name),
			Status:      status,
			RecordCount: fmt.Sprintf("%d", recordCount),
		})
	}
	return res, nil
}

func (r *Route53Provider) DeleteZone(ctx context.Context, id string) error {
	zoneID := id
	if !strings.HasPrefix(zoneID, "/hostedzone/") {
		zoneID = "/hostedzone/" + zoneID
	}
	input := &route53.DeleteHostedZoneInput{
		Id: aws.String(zoneID),
	}
	_, err := r.client.DeleteHostedZone(ctx, input)
	return err
}

func (r *Route53Provider) ListRecords(ctx context.Context, zoneID string) ([]DnsRecord, error) {
	hzID := zoneID
	if !strings.HasPrefix(hzID, "/hostedzone/") {
		hzID = "/hostedzone/" + hzID
	}
	input := &route53.ListResourceRecordSetsInput{
		HostedZoneId: aws.String(hzID),
	}
	output, err := r.client.ListResourceRecordSets(ctx, input)
	if err != nil {
		return nil, err
	}
	var res []DnsRecord
	for _, rrs := range output.ResourceRecordSets {
		ttl := 0
		if rrs.TTL != nil {
			ttl = int(*rrs.TTL)
		}
		var values []string
		for _, val := range rrs.ResourceRecords {
			values = append(values, aws.ToString(val.Value))
		}
		valueStr := strings.Join(values, ", ")
		if rrs.AliasTarget != nil {
			valueStr = fmt.Sprintf("ALIAS %s", aws.ToString(rrs.AliasTarget.DNSName))
		}

		recordID := fmt.Sprintf("%s|%s", aws.ToString(rrs.Name), string(rrs.Type))
		res = append(res, DnsRecord{
			ID:      recordID,
			Name:    aws.ToString(rrs.Name),
			Type:    string(rrs.Type),
			TTL:     ttl,
			Value:   valueStr,
			Proxied: "-",
		})
	}
	return res, nil
}

func (r *Route53Provider) AddRecord(ctx context.Context, zoneID string, record DnsRecord) error {
	hzID := zoneID
	if !strings.HasPrefix(hzID, "/hostedzone/") {
		hzID = "/hostedzone/" + hzID
	}
	var rrs []types.ResourceRecord
	for _, val := range strings.Split(record.Value, ",") {
		val = strings.TrimSpace(val)
		if val != "" {
			rrs = append(rrs, types.ResourceRecord{
				Value: aws.String(val),
			})
		}
	}
	input := &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(hzID),
		ChangeBatch: &types.ChangeBatch{
			Changes: []types.Change{
				{
					Action: types.ChangeActionUpsert,
					ResourceRecordSet: &types.ResourceRecordSet{
						Name:            aws.String(record.Name),
						Type:            types.RRType(strings.ToUpper(record.Type)),
						TTL:             aws.Int64(int64(record.TTL)),
						ResourceRecords: rrs,
					},
				},
			},
		},
	}
	_, err := r.client.ChangeResourceRecordSets(ctx, input)
	return err
}

func (r *Route53Provider) ReplaceRecord(ctx context.Context, zoneID string, record DnsRecord) error {
	return r.AddRecord(ctx, zoneID, record)
}

func (r *Route53Provider) DeleteRecord(ctx context.Context, zoneID string, record DnsRecord) error {
	hzID := zoneID
	if !strings.HasPrefix(hzID, "/hostedzone/") {
		hzID = "/hostedzone/" + hzID
	}

	rVal := record.Value
	rTTL := int64(record.TTL)
	if rVal == "" {
		existingRecords, err := r.ListRecords(ctx, zoneID)
		if err != nil {
			return err
		}
		found := false
		for _, er := range existingRecords {
			eName := strings.TrimSuffix(er.Name, ".")
			qName := strings.TrimSuffix(record.Name, ".")
			if strings.EqualFold(eName, qName) && strings.EqualFold(er.Type, record.Type) {
				rVal = er.Value
				rTTL = int64(er.TTL)
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("no matching record found to delete for %s %s", record.Name, record.Type)
		}
	}

	var rrs []types.ResourceRecord
	for _, val := range strings.Split(rVal, ",") {
		val = strings.TrimSpace(val)
		if val != "" {
			rrs = append(rrs, types.ResourceRecord{
				Value: aws.String(val),
			})
		}
	}

	input := &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(hzID),
		ChangeBatch: &types.ChangeBatch{
			Changes: []types.Change{
				{
					Action: types.ChangeActionDelete,
					ResourceRecordSet: &types.ResourceRecordSet{
						Name:            aws.String(record.Name),
						Type:            types.RRType(strings.ToUpper(record.Type)),
						TTL:             aws.Int64(rTTL),
						ResourceRecords: rrs,
					},
				},
			},
		},
	}
	_, err := r.client.ChangeResourceRecordSets(ctx, input)
	return err
}

// helper to instantiate DnsProvider from config
func newProvider(pc ProviderConfig) (DnsProvider, error) {
	ctx := context.TODO()
	switch strings.ToLower(pc.Type) {
	case "cloudflare":
		var api *cloudflare.API
		var err error
		if pc.Token != "" {
			api, err = cloudflare.NewWithAPIToken(pc.Token)
		} else if pc.Email != "" && pc.Key != "" {
			api, err = cloudflare.New(pc.Key, pc.Email)
		} else {
			return nil, errors.New("missing Cloudflare credentials (token, or email and key)")
		}
		if err != nil {
			return nil, err
		}
		return &CloudflareProvider{
			name:  pc.Name,
			email: pc.Email,
			token: pc.Token,
			key:   pc.Key,
			api:   api,
		}, nil

	case "route53":
		region := pc.Region
		if region == "" {
			region = "us-east-1"
		}
		var cfg aws.Config
		var err error
		if pc.AccessKeyID != "" && pc.SecretAccessKey != "" {
			cfg, err = config.LoadDefaultConfig(ctx,
				config.WithRegion(region),
				config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(pc.AccessKeyID, pc.SecretAccessKey, "")),
			)
		} else {
			cfg, err = config.LoadDefaultConfig(ctx, config.WithRegion(region))
		}
		if err != nil {
			return nil, err
		}
		return &Route53Provider{
			name:   pc.Name,
			client: route53.NewFromConfig(cfg),
		}, nil

	default:
		return nil, fmt.Errorf("unknown provider type %q", pc.Type)
	}
}

// helper to query all providers and unmarshal from Viper
func getProviders() ([]DnsProvider, error) {
	var dnsConf DnsConfig
	if err := viper.UnmarshalKey("dns", &dnsConf); err != nil {
		return nil, err
	}

	var providers []DnsProvider
	for _, pc := range dnsConf.Providers {
		p, err := newProvider(pc)
		if err != nil {
			continue
		}
		providers = append(providers, p)
	}

	// Fallback to environment variables if no providers are explicitly configured
	if len(providers) == 0 {
		cfToken := os.Getenv("CLOUDFLARE_API_TOKEN")
		if cfToken != "" {
			p, err := newProvider(ProviderConfig{
				Name:  "cloudflare-env",
				Type:  "cloudflare",
				Token: cfToken,
				Email: os.Getenv("CLOUDFLARE_API_EMAIL"),
			})
			if err == nil {
				providers = append(providers, p)
			}
		}

		awsKey := os.Getenv("AWS_ACCESS_KEY_ID")
		if awsKey != "" {
			p, err := newProvider(ProviderConfig{
				Name:            "route53-env",
				Type:            "route53",
				AccessKeyID:     awsKey,
				SecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
				Region:          os.Getenv("AWS_REGION"),
			})
			if err == nil {
				providers = append(providers, p)
			}
		}
	}

	if len(providers) == 0 {
		return nil, errors.New("no DNS providers configured. Use 'gsmlg dns add-provider' to register one")
	}

	return providers, nil
}

// helper to search a zone across configured providers
func findZone(ctx context.Context, zoneName string, providerFilter string) (DnsProvider, DnsZone, error) {
	providers, err := getProviders()
	if err != nil {
		return nil, DnsZone{}, err
	}

	var matches []struct {
		p DnsProvider
		z DnsZone
	}

	for _, p := range providers {
		if providerFilter != "" && p.Name() != providerFilter && p.Type() != providerFilter {
			continue
		}
		zones, err := p.ListZones(ctx)
		if err != nil {
			continue
		}
		for _, z := range zones {
			zName := strings.TrimSuffix(z.Name, ".")
			qName := strings.TrimSuffix(zoneName, ".")
			if strings.EqualFold(zName, qName) {
				matches = append(matches, struct {
					p DnsProvider
					z DnsZone
				}{p, z})
			}
		}
	}

	if len(matches) == 0 {
		return nil, DnsZone{}, fmt.Errorf("zone %q not found", zoneName)
	}
	if len(matches) > 1 {
		var names []string
		for _, m := range matches {
			names = append(names, fmt.Sprintf("%s (%s)", m.p.Name(), m.p.Type()))
		}
		return nil, DnsZone{}, fmt.Errorf("multiple providers match zone %q: %s. Please specify a provider using --provider flag", zoneName, strings.Join(names, ", "))
	}

	return matches[0].p, matches[0].z, nil
}

// helper to format BIND zone file export
func exportZone(zoneName string, records []DnsRecord) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("; Zone export for %s\n", zoneName))
	sb.WriteString(fmt.Sprintf("; Generated by gsmlg-cli at %s\n\n", time.Now().Format(time.RFC1123)))

	defaultTTL := 3600
	sb.WriteString(fmt.Sprintf("$TTL %d\n\n", defaultTTL))

	for _, rec := range records {
		name := rec.Name
		if !strings.HasSuffix(name, ".") {
			name = name + "."
		}
		sb.WriteString(fmt.Sprintf("%-30s %-6d IN %-6s %s\n", name, rec.TTL, rec.Type, rec.Value))
	}
	return sb.String()
}

// CLI commands definitions

var dnsCmd = &cobra.Command{
	Use:   "dns",
	Short: "Unified DNS provider management",
	Long:  `Manage DNS zones and records across multiple providers (Cloudflare, AWS Route53).`,
}

var dnsAddProviderCmd = &cobra.Command{
	Use:   "add-provider",
	Short: "Add or update a DNS provider configuration",
	Run: func(cmd *cobra.Command, args []string) {
		providerType, _ := cmd.Flags().GetString("provider")
		if providerType == "" {
			exitIfError(errors.New("--provider is required (cloudflare or route53)"))
		}
		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			name = providerType
		}

		token, _ := cmd.Flags().GetString("token")
		email, _ := cmd.Flags().GetString("email")
		key, _ := cmd.Flags().GetString("key")
		accessKey, _ := cmd.Flags().GetString("access-key-id")
		secretKey, _ := cmd.Flags().GetString("secret-access-key")
		region, _ := cmd.Flags().GetString("region")

		var dnsConf DnsConfig
		_ = viper.UnmarshalKey("dns", &dnsConf)

		newPc := ProviderConfig{
			Name:            name,
			Type:            providerType,
			Token:           token,
			Email:           email,
			Key:             key,
			AccessKeyID:     accessKey,
			SecretAccessKey: secretKey,
			Region:          region,
		}

		found := false
		for i, pc := range dnsConf.Providers {
			if pc.Name == name {
				dnsConf.Providers[i] = newPc
				found = true
				break
			}
		}
		if !found {
			dnsConf.Providers = append(dnsConf.Providers, newPc)
		}

		viper.Set("dns", dnsConf)
		writeConfig()
		fmt.Printf("DNS provider %q (%s) added/updated successfully.\n", name, providerType)
	},
}

var dnsZoneCmd = &cobra.Command{
	Use:   "zone",
	Short: "Manage DNS zones",
}

var dnsZoneListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all zones across providers",
	Run: func(cmd *cobra.Command, args []string) {
		providerFilter, _ := cmd.Flags().GetString("provider")
		providers, err := getProviders()
		exitIfError(err)

		var allZones []DnsZone
		for _, p := range providers {
			if providerFilter != "" && p.Name() != providerFilter && p.Type() != providerFilter {
				continue
			}
			zones, err := p.ListZones(context.TODO())
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to list zones for provider %s: %v\n", p.Name(), err)
				continue
			}
			allZones = append(allZones, zones...)
		}

		ot, _ := cmd.Flags().GetString("output")
		if ot == "json" {
			print.Json(allZones)
		} else {
			print.Table(allZones, []string{"provider", "id", "name", "status", "record_count"})
		}
	},
}

var dnsZoneExportCmd = &cobra.Command{
	Use:   "export [zone-name]",
	Short: "Export a zone to BIND zone file format",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		zoneName := args[0]
		providerFilter, _ := cmd.Flags().GetString("provider")

		p, zone, err := findZone(context.TODO(), zoneName, providerFilter)
		exitIfError(err)

		records, err := p.ListRecords(context.TODO(), zone.ID)
		exitIfError(err)

		exportStr := exportZone(zone.Name, records)
		fmt.Println(exportStr)
	},
}

var dnsRrCmd = &cobra.Command{
	Use:   "rr [zone-name] [action]",
	Short: "Manage resource records in a zone",
	Long:  `Manage resource records in a zone. Actions: list, add, replace, delete.`,
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		zoneName := args[0]
		action := args[1]
		providerFilter, _ := cmd.Flags().GetString("provider")

		p, zone, err := findZone(context.TODO(), zoneName, providerFilter)
		exitIfError(err)

		switch strings.ToLower(action) {
		case "list":
			records, err := p.ListRecords(context.TODO(), zone.ID)
			exitIfError(err)
			ot, _ := cmd.Flags().GetString("output")
			if ot == "json" {
				print.Json(records)
			} else {
				print.Table(records, []string{"id", "name", "type", "content", "ttl", "proxied"})
			}

		case "add", "replace":
			name, _ := cmd.Flags().GetString("name")
			rtype, _ := cmd.Flags().GetString("type")
			rdata, _ := cmd.Flags().GetString("rdata")
			ttl, _ := cmd.Flags().GetInt("ttl")
			proxied, _ := cmd.Flags().GetBool("proxied")

			if name == "" || rtype == "" || rdata == "" {
				exitIfError(errors.New("--name, --type, and --rdata are required for add/replace"))
			}

			proxiedStr := "false"
			if proxied {
				proxiedStr = "true"
			}
			rec := DnsRecord{
				Name:    name,
				Type:    rtype,
				TTL:     ttl,
				Value:   rdata,
				Proxied: proxiedStr,
			}

			if strings.ToLower(action) == "add" {
				err = p.AddRecord(context.TODO(), zone.ID, rec)
			} else {
				err = p.ReplaceRecord(context.TODO(), zone.ID, rec)
			}
			exitIfError(err)
			fmt.Printf("Record %s successfully.\n", action+"ed")

		case "delete":
			id, _ := cmd.Flags().GetString("id")
			name, _ := cmd.Flags().GetString("name")
			rtype, _ := cmd.Flags().GetString("type")
			rdata, _ := cmd.Flags().GetString("rdata")

			if id == "" && (name == "" || rtype == "") {
				exitIfError(errors.New("either --id, or both --name and --type must be provided for delete"))
			}

			rec := DnsRecord{
				ID:    id,
				Name:  name,
				Type:  rtype,
				Value: rdata,
			}
			err = p.DeleteRecord(context.TODO(), zone.ID, rec)
			exitIfError(err)
			fmt.Println("Record deleted successfully.")

		default:
			exitIfError(fmt.Errorf("unknown action %q. Must be list, add, replace, or delete", action))
		}
	},
}

func init() {
	rootCmd.AddCommand(dnsCmd)

	// dns add-provider
	dnsCmd.AddCommand(dnsAddProviderCmd)
	dnsAddProviderCmd.Flags().String("provider", "", "Provider type: cloudflare or route53")
	dnsAddProviderCmd.Flags().String("name", "", "Custom name for this provider config")
	dnsAddProviderCmd.Flags().String("token", "", "Cloudflare API Token")
	dnsAddProviderCmd.Flags().String("email", "", "Cloudflare Email")
	dnsAddProviderCmd.Flags().String("key", "", "Cloudflare API Key")
	dnsAddProviderCmd.Flags().String("access-key-id", "", "AWS Access Key ID")
	dnsAddProviderCmd.Flags().String("secret-access-key", "", "AWS Secret Access Key")
	dnsAddProviderCmd.Flags().String("region", "", "AWS Region")

	// dns zone
	dnsCmd.AddCommand(dnsZoneCmd)
	dnsZoneCmd.PersistentFlags().String("provider", "", "Filter by specific provider name or type")
	dnsZoneCmd.PersistentFlags().StringP("output", "o", "", "Print format: json or plain text")

	dnsZoneCmd.AddCommand(dnsZoneListCmd)
	dnsZoneCmd.AddCommand(dnsZoneExportCmd)

	// dns rr
	dnsCmd.AddCommand(dnsRrCmd)
	dnsRrCmd.PersistentFlags().String("provider", "", "Filter by specific provider name or type")
	dnsRrCmd.PersistentFlags().StringP("output", "o", "", "Print format: json or plain text")

	dnsRrCmd.Flags().String("id", "", "Record ID (use for delete)")
	dnsRrCmd.Flags().StringP("name", "n", "", "Record name")
	dnsRrCmd.Flags().StringP("type", "t", "", "Record type (A, CNAME, TXT, etc.)")
	dnsRrCmd.Flags().Int("ttl", 300, "Record TTL in seconds")
	dnsRrCmd.Flags().StringP("rdata", "d", "", "Record value/data")
	dnsRrCmd.Flags().Bool("proxied", false, "Proxy through Cloudflare (Cloudflare only)")
}
