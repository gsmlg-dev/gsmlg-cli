/*
Copyright © 2022 Jonathan Gao <gsmlg.com@gmail.com>
*/
package cmd

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/spf13/cobra"
)

// publicKeyCmd represents the publicKey command
var publicKeyCmd = &cobra.Command{
	Use:   "public-key",
	Short: "Get public key from private key",
	Long: `Get public key from private key:

Use --private-key or -p to set private key, "-" indicate stdin.`,
	Run: func(cmd *cobra.Command, args []string) {
		key, _ := cmd.Flags().GetString("private-key")
		var f *os.File
		var err error
		if key == "-" {
			f = os.Stdin
		} else {
			f, err = os.Open(key)
			if err != nil {
				log.Fatal(err)
			}
		}
		c, err := io.ReadAll(f)
		if err != nil {
			log.Fatal(err)
		}
		block, _ := pem.Decode(c)
		rsaKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			log.Fatal(err)
		}

		pubKey := x509.MarshalPKCS1PublicKey(&rsaKey.PublicKey)
		pubStr := base64.StdEncoding.EncodeToString(pubKey)
		fmt.Println(pubStr)
	},
}

func init() {
	rsaCmd.AddCommand(publicKeyCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// publicKeyCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	publicKeyCmd.Flags().StringP("private-key", "p", "", "Select private key")
	err := publicKeyCmd.MarkFlagRequired("private-key")
	exitIfError(err)
}
