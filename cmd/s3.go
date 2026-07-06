/*
Copyright © 2026 Jonathan Gao <gsmlg.com@gmail.com>
*/
package cmd

//lint:file-ignore SA1019 Keep the AWS SDK v2 global endpoint resolver until the S3-compatible provider behavior is migrated.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/gsmlg-dev/gsmlg-golang/print"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// AliasConfig represents configuration for an S3 alias
type AliasConfig struct {
	Name            string `mapstructure:"name" yaml:"name" json:"name"`
	Endpoint        string `mapstructure:"endpoint" yaml:"endpoint" json:"endpoint"`
	AccessKeyID     string `mapstructure:"access_key_id" yaml:"access_key_id" json:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key" yaml:"secret_access_key" json:"secret_access_key"`
	Region          string `mapstructure:"region" yaml:"region" json:"region"`
}

// S3Config holds the list of configured S3 aliases
type S3Config struct {
	Aliases []AliasConfig `mapstructure:"aliases" yaml:"aliases"`
}

func getS3Config() (S3Config, error) {
	var s3Conf S3Config
	if err := viper.UnmarshalKey("s3", &s3Conf); err != nil {
		return s3Conf, err
	}
	return s3Conf, nil
}

func saveS3Config(s3Conf S3Config) {
	viper.Set("s3", s3Conf)
	writeConfig()
}

// parseS3Path parses a path like ALIAS/bucket/key.
// If the first component of the path matches a configured S3 alias, it returns the AliasConfig, bucket, key, and true.
// Otherwise it returns empty values and false.
func parseS3Path(pathStr string, aliases []AliasConfig) (ac AliasConfig, bucket string, key string, isS3 bool) {
	if pathStr == "" {
		return AliasConfig{}, "", "", false
	}
	// Normalize path separators
	pathStr = filepath.ToSlash(pathStr)
	parts := strings.Split(pathStr, "/")
	if len(parts) == 0 {
		return AliasConfig{}, "", "", false
	}
	firstPart := parts[0]
	for _, a := range aliases {
		if a.Name == firstPart {
			bucket := ""
			if len(parts) > 1 {
				bucket = parts[1]
			}
			key = ""
			if len(parts) > 2 {
				key = strings.Join(parts[2:], "/")
			}
			return a, bucket, key, true
		}
	}
	return AliasConfig{}, "", "", false
}

func newS3Client(ac AliasConfig) (*s3.Client, error) {
	ctx := context.TODO()
	region := ac.Region
	if region == "" {
		region = "us-east-1"
	}

	var optFns []func(*config.LoadOptions) error
	optFns = append(optFns, config.WithRegion(region))

	if ac.AccessKeyID != "" && ac.SecretAccessKey != "" {
		optFns = append(optFns, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(ac.AccessKeyID, ac.SecretAccessKey, ""),
		))
	}

	isAWSEndpoint := ac.Endpoint != "" && strings.Contains(ac.Endpoint, ".amazonaws.com")
	if ac.Endpoint != "" && !isAWSEndpoint {
		resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) { //nolint:staticcheck // Keep existing S3-compatible endpoint behavior until resolver migration.
			if service == s3.ServiceID {
				return aws.Endpoint{ //nolint:staticcheck // Keep existing S3-compatible endpoint behavior until resolver migration.
					URL:           ac.Endpoint,
					SigningRegion: region,
				}, nil
			}
			return aws.Endpoint{}, &aws.EndpointNotFoundError{} //nolint:staticcheck // Keep existing S3-compatible endpoint behavior until resolver migration.
		})
		optFns = append(optFns, config.WithEndpointResolverWithOptions(resolver))
	}

	cfg, err := config.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if ac.Endpoint != "" && !strings.Contains(ac.Endpoint, ".amazonaws.com") {
			o.UsePathStyle = true
		}
	})

	return client, nil
}

// CLI Command Definitions

var s3Cmd = &cobra.Command{
	Use:   "s3",
	Short: "S3 compatible object storage management (MinIO mc compatible)",
	Long:  `Manage S3/MinIO compatible object storage buckets and files.`,
}

var s3AliasCmd = &cobra.Command{
	Use:   "alias",
	Short: "Manage S3 configuration aliases",
}

var s3AliasSetCmd = &cobra.Command{
	Use:   "set [alias] [endpoint] [access-key] [secret-key]",
	Short: "Set or update S3 configuration alias",
	Args:  cobra.ExactArgs(4),
	Run: func(cmd *cobra.Command, args []string) {
		aliasName := args[0]
		endpoint := args[1]
		accessKey := args[2]
		secretKey := args[3]
		region, _ := cmd.Flags().GetString("region")
		if region == "" {
			region = "us-east-1"
		}

		s3Conf, err := getS3Config()
		exitIfError(err)

		newAlias := AliasConfig{
			Name:            aliasName,
			Endpoint:        endpoint,
			AccessKeyID:     accessKey,
			SecretAccessKey: secretKey,
			Region:          region,
		}

		found := false
		for i, a := range s3Conf.Aliases {
			if a.Name == aliasName {
				s3Conf.Aliases[i] = newAlias
				found = true
				break
			}
		}
		if !found {
			s3Conf.Aliases = append(s3Conf.Aliases, newAlias)
		}

		saveS3Config(s3Conf)
		fmt.Printf("Added alias %q successfully.\n", aliasName)
	},
}

var s3AliasListCmd = &cobra.Command{
	Use:   "list",
	Short: "List S3 configuration aliases",
	Run: func(cmd *cobra.Command, args []string) {
		s3Conf, err := getS3Config()
		exitIfError(err)

		if len(s3Conf.Aliases) == 0 {
			fmt.Println("No S3 aliases configured.")
			return
		}

		type AliasDisplay struct {
			Alias     string `json:"alias"`
			Endpoint  string `json:"endpoint"`
			AccessKey string `json:"access_key"`
			Region    string `json:"region"`
		}

		var list []AliasDisplay
		for _, a := range s3Conf.Aliases {
			maskedAccess := a.AccessKeyID
			if len(maskedAccess) > 8 {
				maskedAccess = maskedAccess[:4] + "..." + maskedAccess[len(maskedAccess)-4:]
			}
			list = append(list, AliasDisplay{
				Alias:     a.Name,
				Endpoint:  a.Endpoint,
				AccessKey: maskedAccess,
				Region:    a.Region,
			})
		}

		print.Table(list, []string{"alias", "endpoint", "access_key", "region"})
	},
}

var s3AliasRemoveCmd = &cobra.Command{
	Use:     "remove [alias]",
	Aliases: []string{"rm"},
	Short:   "Remove S3 configuration alias",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		aliasName := args[0]
		s3Conf, err := getS3Config()
		exitIfError(err)

		foundIndex := -1
		for i, a := range s3Conf.Aliases {
			if a.Name == aliasName {
				foundIndex = i
				break
			}
		}

		if foundIndex == -1 {
			exitIfError(fmt.Errorf("alias %q not found", aliasName))
		}

		s3Conf.Aliases = append(s3Conf.Aliases[:foundIndex], s3Conf.Aliases[foundIndex+1:]...)
		saveS3Config(s3Conf)
		fmt.Printf("Removed alias %q successfully.\n", aliasName)
	},
}

var s3LsCmd = &cobra.Command{
	Use:   "ls [ALIAS/[bucket]/[prefix]]",
	Short: "List aliases, buckets, or objects",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		s3Conf, err := getS3Config()
		exitIfError(err)

		if len(args) == 0 {
			// List all aliases
			if len(s3Conf.Aliases) == 0 {
				fmt.Println("No S3 aliases configured.")
				return
			}
			for _, a := range s3Conf.Aliases {
				fmt.Printf("%s/\n", a.Name)
			}
			return
		}

		ac, bucket, key, isS3 := parseS3Path(args[0], s3Conf.Aliases)
		if !isS3 {
			exitIfError(fmt.Errorf("invalid S3 path %q (must start with a configured alias)", args[0]))
		}

		client, err := newS3Client(ac)
		exitIfError(err)

		ctx := context.TODO()

		if bucket == "" {
			// List buckets
			output, err := client.ListBuckets(ctx, &s3.ListBucketsInput{})
			exitIfError(err)

			for _, b := range output.Buckets {
				creationDate := ""
				if b.CreationDate != nil {
					creationDate = b.CreationDate.Format("2006-01-02 15:04:05")
				}
				fmt.Printf("[%s] %s/\n", creationDate, aws.ToString(b.Name))
			}
			return
		}

		// List objects in bucket matching prefix
		input := &s3.ListObjectsV2Input{
			Bucket:    aws.String(bucket),
			Prefix:    aws.String(key),
			Delimiter: aws.String("/"),
		}

		paginator := s3.NewListObjectsV2Paginator(client, input)
		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			exitIfError(err)

			// Print prefixes (folders)
			for _, cp := range page.CommonPrefixes {
				fmt.Printf("%-20s %10s %s\n", "-", "DIR", aws.ToString(cp.Prefix))
			}

			// Print objects
			for _, obj := range page.Contents {
				lastMod := ""
				if obj.LastModified != nil {
					lastMod = obj.LastModified.Format("2006-01-02 15:04:05")
				}
				sizeStr := formatBytes(aws.ToInt64(obj.Size))
				fmt.Printf("[%s] %10s %s\n", lastMod, sizeStr, aws.ToString(obj.Key))
			}
		}
	},
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

var s3MbCmd = &cobra.Command{
	Use:   "mb [ALIAS/bucket]",
	Short: "Make a bucket",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		s3Conf, err := getS3Config()
		exitIfError(err)

		ac, bucket, _, isS3 := parseS3Path(args[0], s3Conf.Aliases)
		if !isS3 || bucket == "" {
			exitIfError(fmt.Errorf("invalid S3 path %q (must be in format ALIAS/bucket)", args[0]))
		}

		client, err := newS3Client(ac)
		exitIfError(err)

		input := &s3.CreateBucketInput{
			Bucket: aws.String(bucket),
		}

		region := ac.Region
		if region == "" {
			region = "us-east-1"
		}
		if region != "us-east-1" {
			input.CreateBucketConfiguration = &types.CreateBucketConfiguration{
				LocationConstraint: types.BucketLocationConstraint(region),
			}
		}

		_, err = client.CreateBucket(context.TODO(), input)
		exitIfError(err)

		fmt.Printf("Bucket %q created successfully.\n", bucket)
	},
}

var s3RbCmd = &cobra.Command{
	Use:   "rb [ALIAS/bucket]",
	Short: "Remove a bucket",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		s3Conf, err := getS3Config()
		exitIfError(err)

		ac, bucket, _, isS3 := parseS3Path(args[0], s3Conf.Aliases)
		if !isS3 || bucket == "" {
			exitIfError(fmt.Errorf("invalid S3 path %q (must be in format ALIAS/bucket)", args[0]))
		}

		force, _ := cmd.Flags().GetBool("force")

		client, err := newS3Client(ac)
		exitIfError(err)

		ctx := context.TODO()

		if force {
			paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
				Bucket: aws.String(bucket),
			})

			for paginator.HasMorePages() {
				page, err := paginator.NextPage(ctx)
				exitIfError(err)

				if len(page.Contents) > 0 {
					var objectIds []types.ObjectIdentifier
					for _, obj := range page.Contents {
						objectIds = append(objectIds, types.ObjectIdentifier{
							Key: obj.Key,
						})
					}

					_, err = client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
						Bucket: aws.String(bucket),
						Delete: &types.Delete{
							Objects: objectIds,
							Quiet:   aws.Bool(true),
						},
					})
					exitIfError(err)
				}
			}
		}

		_, err = client.DeleteBucket(ctx, &s3.DeleteBucketInput{
			Bucket: aws.String(bucket),
		})
		exitIfError(err)

		fmt.Printf("Bucket %q removed successfully.\n", bucket)
	},
}

var s3RmCmd = &cobra.Command{
	Use:   "rm [ALIAS/bucket/object]",
	Short: "Remove objects",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		s3Conf, err := getS3Config()
		exitIfError(err)

		ac, bucket, key, isS3 := parseS3Path(args[0], s3Conf.Aliases)
		if !isS3 || bucket == "" || key == "" {
			exitIfError(fmt.Errorf("invalid S3 path %q (must be in format ALIAS/bucket/object)", args[0]))
		}

		recursive, _ := cmd.Flags().GetBool("recursive")

		client, err := newS3Client(ac)
		exitIfError(err)

		ctx := context.TODO()

		if recursive {
			paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
				Bucket: aws.String(bucket),
				Prefix: aws.String(key),
			})

			deletedCount := 0
			for paginator.HasMorePages() {
				page, err := paginator.NextPage(ctx)
				exitIfError(err)

				if len(page.Contents) > 0 {
					var objectIds []types.ObjectIdentifier
					for _, obj := range page.Contents {
						objectIds = append(objectIds, types.ObjectIdentifier{
							Key: obj.Key,
						})
					}

					output, err := client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
						Bucket: aws.String(bucket),
						Delete: &types.Delete{
							Objects: objectIds,
							Quiet:   aws.Bool(true),
						},
					})
					exitIfError(err)
					deletedCount += len(objectIds)
					for _, obj := range output.Deleted {
						fmt.Printf("Deleted %s\n", aws.ToString(obj.Key))
					}
				}
			}
			fmt.Printf("Deleted %d objects recursively.\n", deletedCount)
		} else {
			_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
			})
			exitIfError(err)
			fmt.Printf("Deleted %q.\n", key)
		}
	},
}

var s3CatCmd = &cobra.Command{
	Use:   "cat [ALIAS/bucket/object]",
	Short: "Cat object contents",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		s3Conf, err := getS3Config()
		exitIfError(err)

		ac, bucket, key, isS3 := parseS3Path(args[0], s3Conf.Aliases)
		if !isS3 || bucket == "" || key == "" {
			exitIfError(fmt.Errorf("invalid S3 path %q (must be in format ALIAS/bucket/object)", args[0]))
		}

		client, err := newS3Client(ac)
		exitIfError(err)

		output, err := client.GetObject(context.TODO(), &s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		exitIfError(err)
		defer output.Body.Close()

		_, err = io.Copy(os.Stdout, output.Body)
		exitIfError(err)
	},
}

var s3ShareCmd = &cobra.Command{
	Use:   "share [ALIAS/bucket/object]",
	Short: "Generate pre-signed URL to share object",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		s3Conf, err := getS3Config()
		exitIfError(err)

		ac, bucket, key, isS3 := parseS3Path(args[0], s3Conf.Aliases)
		if !isS3 || bucket == "" || key == "" {
			exitIfError(fmt.Errorf("invalid S3 path %q (must be in format ALIAS/bucket/object)", args[0]))
		}

		expireStr, _ := cmd.Flags().GetString("expire")
		expireDur := 168 * time.Hour
		if expireStr != "" {
			var err error
			expireDur, err = time.ParseDuration(expireStr)
			exitIfError(err)
		}

		client, err := newS3Client(ac)
		exitIfError(err)

		presignClient := s3.NewPresignClient(client)

		req, err := presignClient.PresignGetObject(context.TODO(), &s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		}, s3.WithPresignExpires(expireDur))
		exitIfError(err)

		fmt.Println(req.URL)
	},
}

type PathType int

const (
	PathLocal PathType = iota
	PathS3
)

type ResolvedPath struct {
	Type   PathType
	Raw    string
	Alias  AliasConfig
	Bucket string
	Key    string
}

func resolvePath(pathStr string, aliases []AliasConfig) ResolvedPath {
	ac, bucket, key, isS3 := parseS3Path(pathStr, aliases)
	if isS3 {
		return ResolvedPath{
			Type:   PathS3,
			Raw:    pathStr,
			Alias:  ac,
			Bucket: bucket,
			Key:    key,
		}
	}
	return ResolvedPath{
		Type: PathLocal,
		Raw:  pathStr,
	}
}

var s3CpCmd = &cobra.Command{
	Use:   "cp [source] [target]",
	Short: "Copy objects and files",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		s3Conf, err := getS3Config()
		exitIfError(err)

		recursive, _ := cmd.Flags().GetBool("recursive")

		src := resolvePath(args[0], s3Conf.Aliases)
		dst := resolvePath(args[1], s3Conf.Aliases)

		ctx := context.TODO()

		if src.Type == PathLocal && dst.Type == PathLocal {
			exitIfError(errors.New("both source and target are local (use standard cp command)"))
		}

		if src.Type == PathLocal && dst.Type == PathS3 {
			client, err := newS3Client(dst.Alias)
			exitIfError(err)

			if recursive {
				err = uploadRecursive(ctx, client, src.Raw, dst.Bucket, dst.Key)
				exitIfError(err)
			} else {
				destKey := dst.Key
				if destKey == "" || strings.HasSuffix(destKey, "/") {
					destKey = destKey + filepath.Base(src.Raw)
				}
				err = uploadFile(ctx, client, src.Raw, dst.Bucket, destKey)
				exitIfError(err)
				fmt.Printf("Uploaded %s to S3://%s/%s\n", src.Raw, dst.Bucket, destKey)
			}
		} else if src.Type == PathS3 && dst.Type == PathLocal {
			client, err := newS3Client(src.Alias)
			exitIfError(err)

			if recursive {
				err = downloadRecursive(ctx, client, src.Bucket, src.Key, dst.Raw)
				exitIfError(err)
			} else {
				localPath := dst.Raw
				fi, err := os.Stat(localPath)
				isDir := false
				if err == nil && fi.IsDir() {
					isDir = true
				} else if strings.HasSuffix(localPath, "/") {
					isDir = true
				}

				if isDir {
					if err != nil {
						err = os.MkdirAll(localPath, 0755)
						exitIfError(err)
					}
					localPath = filepath.Join(localPath, filepath.Base(src.Key))
				}
				err = downloadFile(ctx, client, src.Bucket, src.Key, localPath)
				exitIfError(err)
				fmt.Printf("Downloaded S3://%s/%s to %s\n", src.Bucket, src.Key, localPath)
			}
		} else if src.Type == PathS3 && dst.Type == PathS3 {
			srcClient, err := newS3Client(src.Alias)
			exitIfError(err)

			dstClient, err := newS3Client(dst.Alias)
			exitIfError(err)

			sameProvider := (src.Alias.Endpoint == dst.Alias.Endpoint && src.Alias.Region == dst.Alias.Region)

			if recursive {
				err = copyS3ToS3Recursive(ctx, srcClient, src.Bucket, src.Key, dstClient, dst.Bucket, dst.Key, sameProvider)
				exitIfError(err)
			} else {
				destKey := dst.Key
				if destKey == "" || strings.HasSuffix(destKey, "/") {
					destKey = destKey + filepath.Base(src.Key)
				}
				err = copyS3ToS3(ctx, srcClient, src.Bucket, src.Key, dstClient, dst.Bucket, destKey, sameProvider)
				exitIfError(err)
				fmt.Printf("Copied S3://%s/%s to S3://%s/%s\n", src.Bucket, src.Key, dst.Bucket, destKey)
			}
		}
	},
}

func uploadFile(ctx context.Context, client *s3.Client, localPath string, bucket string, key string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	fi, err := file.Stat()
	if err != nil {
		return err
	}

	size := fi.Size()

	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(key),
		Body:          file,
		ContentLength: aws.Int64(size),
	})
	return err
}

func uploadRecursive(ctx context.Context, client *s3.Client, localDir string, bucket string, keyPrefix string) error {
	localDirClean := filepath.Clean(localDir)
	err := filepath.Walk(localDirClean, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(localDirClean, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		destKey := ""
		if keyPrefix == "" {
			destKey = relPath
		} else if strings.HasSuffix(keyPrefix, "/") {
			destKey = keyPrefix + relPath
		} else {
			destKey = keyPrefix + "/" + relPath
		}

		fmt.Printf("Uploading %s -> S3://%s/%s\n", path, bucket, destKey)
		return uploadFile(ctx, client, path, bucket, destKey)
	})
	return err
}

func downloadFile(ctx context.Context, client *s3.Client, bucket string, key string, localPath string) error {
	dir := filepath.Dir(localPath)
	if dir != "." && dir != "/" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	output, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return err
	}
	defer output.Body.Close()

	file, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, output.Body)
	return err
}

func downloadRecursive(ctx context.Context, client *s3.Client, bucket string, keyPrefix string, localDir string) error {
	paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(keyPrefix),
	})

	basePrefix := keyPrefix
	if !strings.HasSuffix(basePrefix, "/") {
		if idx := strings.LastIndex(basePrefix, "/"); idx != -1 {
			basePrefix = basePrefix[:idx+1]
		} else {
			basePrefix = ""
		}
	}

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return err
		}

		for _, obj := range page.Contents {
			key := aws.ToString(obj.Key)
			if strings.HasSuffix(key, "/") {
				continue
			}

			relPath := strings.TrimPrefix(key, basePrefix)
			localPath := filepath.Join(localDir, filepath.FromSlash(relPath))

			fmt.Printf("Downloading S3://%s/%s -> %s\n", bucket, key, localPath)
			err = downloadFile(ctx, client, bucket, key, localPath)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func copyS3ToS3(ctx context.Context, srcClient *s3.Client, srcBucket, srcKey string, dstClient *s3.Client, dstBucket, dstKey string, sameProvider bool) error {
	if sameProvider {
		copySource := srcBucket + "/" + srcKey
		_, err := dstClient.CopyObject(ctx, &s3.CopyObjectInput{
			Bucket:     aws.String(dstBucket),
			Key:        aws.String(dstKey),
			CopySource: aws.String(copySource),
		})
		return err
	} else {
		getOut, err := srcClient.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(srcBucket),
			Key:    aws.String(srcKey),
		})
		if err != nil {
			return err
		}
		defer getOut.Body.Close()

		_, err = dstClient.PutObject(ctx, &s3.PutObjectInput{
			Bucket:        aws.String(dstBucket),
			Key:           aws.String(dstKey),
			Body:          getOut.Body,
			ContentLength: getOut.ContentLength,
		})
		return err
	}
}

func copyS3ToS3Recursive(ctx context.Context, srcClient *s3.Client, srcBucket, srcPrefix string, dstClient *s3.Client, dstBucket, dstPrefix string, sameProvider bool) error {
	paginator := s3.NewListObjectsV2Paginator(srcClient, &s3.ListObjectsV2Input{
		Bucket: aws.String(srcBucket),
		Prefix: aws.String(srcPrefix),
	})

	basePrefix := srcPrefix
	if !strings.HasSuffix(basePrefix, "/") {
		if idx := strings.LastIndex(basePrefix, "/"); idx != -1 {
			basePrefix = basePrefix[:idx+1]
		} else {
			basePrefix = ""
		}
	}

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return err
		}

		for _, obj := range page.Contents {
			key := aws.ToString(obj.Key)
			if strings.HasSuffix(key, "/") {
				continue
			}

			relPath := strings.TrimPrefix(key, basePrefix)
			destKey := dstPrefix
			if destKey == "" {
				destKey = relPath
			} else if strings.HasSuffix(destKey, "/") {
				destKey = destKey + relPath
			} else {
				destKey = destKey + "/" + relPath
			}

			fmt.Printf("Copying S3://%s/%s -> S3://%s/%s\n", srcBucket, key, dstBucket, destKey)
			err = copyS3ToS3(ctx, srcClient, srcBucket, key, dstClient, dstBucket, destKey, sameProvider)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func init() {
	rootCmd.AddCommand(s3Cmd)

	// alias
	s3Cmd.AddCommand(s3AliasCmd)
	s3AliasCmd.AddCommand(s3AliasSetCmd)
	s3AliasSetCmd.Flags().String("region", "us-east-1", "AWS S3 region")
	s3AliasCmd.AddCommand(s3AliasListCmd)
	s3AliasCmd.AddCommand(s3AliasRemoveCmd)

	// ls
	s3Cmd.AddCommand(s3LsCmd)

	// mb
	s3Cmd.AddCommand(s3MbCmd)

	// rb
	s3Cmd.AddCommand(s3RbCmd)
	s3RbCmd.Flags().Bool("force", false, "Force delete all objects in bucket before removal")

	// rm
	s3Cmd.AddCommand(s3RmCmd)
	s3RmCmd.Flags().Bool("recursive", false, "Remove recursively")

	// cp
	s3Cmd.AddCommand(s3CpCmd)
	s3CpCmd.Flags().Bool("recursive", false, "Copy recursively")

	// mv
	s3Cmd.AddCommand(s3MvCmd)
	s3MvCmd.Flags().Bool("recursive", false, "Move recursively")

	// cat
	s3Cmd.AddCommand(s3CatCmd)

	// head
	s3Cmd.AddCommand(s3HeadCmd)

	// share
	s3Cmd.AddCommand(s3ShareCmd)
	s3ShareCmd.Flags().String("expire", "168h", "Pre-signed URL expiry duration (e.g. 1h, 24h, 7d)")

	// find
	s3Cmd.AddCommand(s3FindCmd)
	s3FindCmd.Flags().String("name", "", "Filter objects by name glob pattern (e.g. '*.log')")
	s3FindCmd.Flags().String("newer-than", "", "Only show objects newer than duration (e.g. 24h, 7d)")
	s3FindCmd.Flags().String("older-than", "", "Only show objects older than duration (e.g. 24h, 7d)")
	s3FindCmd.Flags().Int("maxdepth", 0, "Maximum search depth (0 = unlimited)")

	// tree
	s3Cmd.AddCommand(s3TreeCmd)
	s3TreeCmd.Flags().Int("maxdepth", 0, "Maximum depth to display (0 = unlimited)")
}

// ---------------------------------------------------------------------------
// mv: copy then delete source
// ---------------------------------------------------------------------------

var s3MvCmd = &cobra.Command{
	Use:   "mv [source] [target]",
	Short: "Move objects (copy then delete source)",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		s3Conf, err := getS3Config()
		exitIfError(err)

		recursive, _ := cmd.Flags().GetBool("recursive")

		src := resolvePath(args[0], s3Conf.Aliases)
		dst := resolvePath(args[1], s3Conf.Aliases)

		ctx := context.TODO()

		if src.Type == PathLocal && dst.Type == PathLocal {
			exitIfError(errors.New("both source and target are local — use the system mv command"))
		}

		// ---- local → S3 ----
		if src.Type == PathLocal && dst.Type == PathS3 {
			client, err := newS3Client(dst.Alias)
			exitIfError(err)

			if recursive {
				exitIfError(uploadRecursive(ctx, client, src.Raw, dst.Bucket, dst.Key))
				exitIfError(os.RemoveAll(src.Raw))
			} else {
				destKey := dst.Key
				if destKey == "" || strings.HasSuffix(destKey, "/") {
					destKey += filepath.Base(src.Raw)
				}
				exitIfError(uploadFile(ctx, client, src.Raw, dst.Bucket, destKey))
				exitIfError(os.Remove(src.Raw))
				fmt.Printf("Moved %s to S3://%s/%s\n", src.Raw, dst.Bucket, destKey)
			}
			return
		}

		// ---- S3 → local ----
		if src.Type == PathS3 && dst.Type == PathLocal {
			client, err := newS3Client(src.Alias)
			exitIfError(err)

			if recursive {
				exitIfError(downloadRecursive(ctx, client, src.Bucket, src.Key, dst.Raw))
				// delete all source objects
				paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
					Bucket: aws.String(src.Bucket),
					Prefix: aws.String(src.Key),
				})
				for paginator.HasMorePages() {
					page, err := paginator.NextPage(ctx)
					exitIfError(err)
					if len(page.Contents) == 0 {
						continue
					}
					var ids []types.ObjectIdentifier
					for _, obj := range page.Contents {
						ids = append(ids, types.ObjectIdentifier{Key: obj.Key})
					}
					_, err = client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
						Bucket: aws.String(src.Bucket),
						Delete: &types.Delete{Objects: ids, Quiet: aws.Bool(true)},
					})
					exitIfError(err)
				}
			} else {
				localPath := dst.Raw
				fi, statErr := os.Stat(localPath)
				if statErr == nil && fi.IsDir() || strings.HasSuffix(localPath, "/") {
					if statErr != nil {
						exitIfError(os.MkdirAll(localPath, 0755))
					}
					localPath = filepath.Join(localPath, filepath.Base(src.Key))
				}
				exitIfError(downloadFile(ctx, client, src.Bucket, src.Key, localPath))
				_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
					Bucket: aws.String(src.Bucket),
					Key:    aws.String(src.Key),
				})
				exitIfError(err)
				fmt.Printf("Moved S3://%s/%s to %s\n", src.Bucket, src.Key, localPath)
			}
			return
		}

		// ---- S3 → S3 ----
		srcClient, err := newS3Client(src.Alias)
		exitIfError(err)
		dstClient, err := newS3Client(dst.Alias)
		exitIfError(err)
		sameProvider := src.Alias.Endpoint == dst.Alias.Endpoint && src.Alias.Region == dst.Alias.Region

		if recursive {
			exitIfError(copyS3ToS3Recursive(ctx, srcClient, src.Bucket, src.Key, dstClient, dst.Bucket, dst.Key, sameProvider))
			// delete all source objects
			paginator := s3.NewListObjectsV2Paginator(srcClient, &s3.ListObjectsV2Input{
				Bucket: aws.String(src.Bucket),
				Prefix: aws.String(src.Key),
			})
			for paginator.HasMorePages() {
				page, err := paginator.NextPage(ctx)
				exitIfError(err)
				if len(page.Contents) == 0 {
					continue
				}
				var ids []types.ObjectIdentifier
				for _, obj := range page.Contents {
					ids = append(ids, types.ObjectIdentifier{Key: obj.Key})
				}
				_, err = srcClient.DeleteObjects(ctx, &s3.DeleteObjectsInput{
					Bucket: aws.String(src.Bucket),
					Delete: &types.Delete{Objects: ids, Quiet: aws.Bool(true)},
				})
				exitIfError(err)
			}
		} else {
			destKey := dst.Key
			if destKey == "" || strings.HasSuffix(destKey, "/") {
				destKey += filepath.Base(src.Key)
			}
			exitIfError(copyS3ToS3(ctx, srcClient, src.Bucket, src.Key, dstClient, dst.Bucket, destKey, sameProvider))
			_, err = srcClient.DeleteObject(ctx, &s3.DeleteObjectInput{
				Bucket: aws.String(src.Bucket),
				Key:    aws.String(src.Key),
			})
			exitIfError(err)
			fmt.Printf("Moved S3://%s/%s to S3://%s/%s\n", src.Bucket, src.Key, dst.Bucket, destKey)
		}
	},
}

// ---------------------------------------------------------------------------
// head: show object metadata
// ---------------------------------------------------------------------------

var s3HeadCmd = &cobra.Command{
	Use:   "head [ALIAS/bucket/object]",
	Short: "Display object metadata",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		s3Conf, err := getS3Config()
		exitIfError(err)

		ac, bucket, key, isS3 := parseS3Path(args[0], s3Conf.Aliases)
		if !isS3 || bucket == "" || key == "" {
			exitIfError(fmt.Errorf("invalid S3 path %q (must be in format ALIAS/bucket/object)", args[0]))
		}

		client, err := newS3Client(ac)
		exitIfError(err)

		out, err := client.HeadObject(context.TODO(), &s3.HeadObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		exitIfError(err)

		lastMod := ""
		if out.LastModified != nil {
			lastMod = out.LastModified.Format(time.RFC3339)
		}
		contentType := aws.ToString(out.ContentType)
		etag := strings.Trim(aws.ToString(out.ETag), "\"")
		size := aws.ToInt64(out.ContentLength)

		fmt.Printf("Key          : %s\n", key)
		fmt.Printf("Bucket       : %s\n", bucket)
		fmt.Printf("Size         : %s (%d bytes)\n", formatBytes(size), size)
		fmt.Printf("Content-Type : %s\n", contentType)
		fmt.Printf("ETag         : %s\n", etag)
		fmt.Printf("Last-Modified: %s\n", lastMod)

		if len(out.Metadata) > 0 {
			fmt.Println("Metadata     :")
			for k, v := range out.Metadata {
				fmt.Printf("  %s: %s\n", k, v)
			}
		}
	},
}

// ---------------------------------------------------------------------------
// find: search objects with filters
// ---------------------------------------------------------------------------

// parseDuration parses a duration string that also accepts "d" for days.
func parseDuration(s string) (time.Duration, error) {
	// support "7d" style input
	if strings.HasSuffix(s, "d") {
		days := strings.TrimSuffix(s, "d")
		n := 0
		if _, err := fmt.Sscanf(days, "%d", &n); err != nil {
			return 0, fmt.Errorf("invalid duration %q", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

// matchGlob reports whether name matches the glob pattern.
// It uses filepath.Match which supports *, ?, and [...] patterns.
func matchGlob(pattern, name string) (bool, error) {
	if pattern == "" {
		return true, nil
	}
	return filepath.Match(pattern, filepath.Base(name))
}

var s3FindCmd = &cobra.Command{
	Use:   "find [ALIAS/bucket/[prefix]]",
	Short: "Find objects matching criteria",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		s3Conf, err := getS3Config()
		exitIfError(err)

		ac, bucket, prefix, isS3 := parseS3Path(args[0], s3Conf.Aliases)
		if !isS3 || bucket == "" {
			exitIfError(fmt.Errorf("invalid S3 path %q (must be in format ALIAS/bucket[/prefix])", args[0]))
		}

		namePattern, _ := cmd.Flags().GetString("name")
		newerStr, _ := cmd.Flags().GetString("newer-than")
		olderStr, _ := cmd.Flags().GetString("older-than")
		maxdepth, _ := cmd.Flags().GetInt("maxdepth")

		var newerThan, olderThan time.Duration
		if newerStr != "" {
			newerThan, err = parseDuration(newerStr)
			exitIfError(err)
		}
		if olderStr != "" {
			olderThan, err = parseDuration(olderStr)
			exitIfError(err)
		}

		client, err := newS3Client(ac)
		exitIfError(err)

		ctx := context.TODO()
		now := time.Now()

		paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
			Bucket: aws.String(bucket),
			Prefix: aws.String(prefix),
		})

		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			exitIfError(err)

			for _, obj := range page.Contents {
				key := aws.ToString(obj.Key)
				if strings.HasSuffix(key, "/") {
					continue
				}

				// depth check: count '/' separators relative to prefix
				if maxdepth > 0 {
					rel := strings.TrimPrefix(key, prefix)
					depth := strings.Count(rel, "/") + 1
					if depth > maxdepth {
						continue
					}
				}

				// name glob filter
				matched, err := matchGlob(namePattern, key)
				exitIfError(err)
				if !matched {
					continue
				}

				// time filters
				if obj.LastModified != nil {
					age := now.Sub(*obj.LastModified)
					if newerThan > 0 && age > newerThan {
						continue
					}
					if olderThan > 0 && age < olderThan {
						continue
					}
				}

				fmt.Printf("%s/%s/%s\n", ac.Name, bucket, key)
			}
		}
	},
}

// ---------------------------------------------------------------------------
// tree: render bucket/prefix as a visual directory tree
// ---------------------------------------------------------------------------

var s3TreeCmd = &cobra.Command{
	Use:   "tree [ALIAS/bucket/[prefix]]",
	Short: "List objects as a directory tree",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		s3Conf, err := getS3Config()
		exitIfError(err)

		ac, bucket, prefix, isS3 := parseS3Path(args[0], s3Conf.Aliases)
		if !isS3 || bucket == "" {
			exitIfError(fmt.Errorf("invalid S3 path %q (must be in format ALIAS/bucket[/prefix])", args[0]))
		}

		maxdepth, _ := cmd.Flags().GetInt("maxdepth")

		client, err := newS3Client(ac)
		exitIfError(err)

		ctx := context.TODO()

		// Collect all keys
		var allKeys []string
		paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
			Bucket: aws.String(bucket),
			Prefix: aws.String(prefix),
		})
		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			exitIfError(err)
			for _, obj := range page.Contents {
				key := aws.ToString(obj.Key)
				if !strings.HasSuffix(key, "/") {
					allKeys = append(allKeys, strings.TrimPrefix(key, prefix))
				}
			}
		}

		// Print root
		rootLabel := fmt.Sprintf("%s/%s", ac.Name, bucket)
		if prefix != "" {
			rootLabel += "/" + prefix
		}
		fmt.Println(rootLabel)

		// Build and render a simple prefix tree
		printTree(allKeys, prefix, "", maxdepth, 0)
	},
}

// printTree renders lines as an indented tree. keys are relative to the
// current prefix. indent is the visual prefix string (e.g. "│   ├── ").
func printTree(keys []string, rootPrefix, indent string, maxdepth, depth int) {
	if maxdepth > 0 && depth >= maxdepth {
		return
	}

	// Group entries by their first path segment
	type entry struct {
		segment  string
		isFile   bool
		children []string
	}

	seen := map[string]*entry{}
	var order []string

	for _, k := range keys {
		parts := strings.SplitN(k, "/", 2)
		seg := parts[0]
		if _, exists := seen[seg]; !exists {
			seen[seg] = &entry{segment: seg}
			order = append(order, seg)
		}
		if len(parts) == 1 {
			seen[seg].isFile = true
		} else {
			seen[seg].children = append(seen[seg].children, parts[1])
		}
	}

	for i, seg := range order {
		e := seen[seg]
		connector := "├── "
		childIndent := indent + "│   "
		if i == len(order)-1 {
			connector = "└── "
			childIndent = indent + "    "
		}
		if e.isFile || len(e.children) == 0 {
			fmt.Printf("%s%s%s\n", indent, connector, seg)
		} else {
			fmt.Printf("%s%s%s/\n", indent, connector, seg)
		}
		if len(e.children) > 0 {
			printTree(e.children, "", childIndent, maxdepth, depth+1)
		}
	}
}
