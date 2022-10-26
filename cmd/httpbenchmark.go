/*
Copyright © 2022 Jonathan Gao <gsmlg.com@gmail.com>
*/
package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/gsmlg-dev/gsmlg-golang/errorhandler"
	"github.com/gsmlg-dev/gsmlg-golang/req"
	"github.com/spf13/cobra"
)

// httpbenchmarkCmd represents the httpbenchmark command
var httpbenchmarkCmd = &cobra.Command{
	Use:   "httpbenchmark",
	Short: "httpbenchmark tool",
	Long: `Run http benchmark easy:
	
	gsmlg-cli httpbenchmark [flags] [url]`,
	Run: func(cmd *cobra.Command, args []string) {
		errAndExit := errorhandler.CreateExitIfError("GSMLG bench: ")

		concurrency, _ := cmd.Flags().GetInt("concurrency")
		requests, _ := cmd.Flags().GetInt64("requests")
		duration, _ := cmd.Flags().GetDuration("duration")
		interval, _ := cmd.Flags().GetDuration("interval")
		seconds, _ := cmd.Flags().GetBool("seconds")

		body, _ := cmd.Flags().GetString("body")
		stream, _ := cmd.Flags().GetBool("stream")
		method, _ := cmd.Flags().GetString("method")
		headers, _ := cmd.Flags().GetStringSlice("header")
		host, _ := cmd.Flags().GetString("host")
		contentType, _ := cmd.Flags().GetString("content")
		cert, _ := cmd.Flags().GetString("cert")
		key, _ := cmd.Flags().GetString("key")
		insecure, _ := cmd.Flags().GetBool("insecure")

		timeout, _ := cmd.Flags().GetDuration("timeout")
		dialTimeout, _ := cmd.Flags().GetDuration("dial-timeout")
		reqWriteTimeout, _ := cmd.Flags().GetDuration("req-timeout")
		respReadTimeout, _ := cmd.Flags().GetDuration("resp-timeout")
		socks5, _ := cmd.Flags().GetString("socks5")

		clean, _ := cmd.Flags().GetBool("clean")
		summary, _ := cmd.Flags().GetBool("summary")
		url := args[0]

		if url == "" {
			errAndExit("url is required")
			return
		}

		if requests >= 0 && requests < int64(concurrency) {
			errAndExit("requests must greater than or equal concurrency")
			return
		}
		if (cert != "" && key == "") || (cert == "" && key != "") {
			errAndExit("must specify cert and key at the same time")
			return
		}

		var err error
		var bodyBytes []byte
		var bodyFile string
		if strings.HasPrefix(body, "@") {
			fileName := (body)[1:]
			if _, err = os.Stat(fileName); err != nil {
				errAndExit(err.Error())
				return
			}
			if stream {
				bodyFile = fileName
			} else {
				bodyBytes, err = ioutil.ReadFile(fileName)
				if err != nil {
					errAndExit(err.Error())
					return
				}
			}
		} else if body != "" {
			bodyBytes = []byte(body)
		}

		clientOpt := req.ClientOpt{
			Url:       url,
			Method:    method,
			Headers:   headers,
			BodyBytes: bodyBytes,
			BodyFile:  bodyFile,

			CertPath: cert,
			KeyPath:  key,
			Insecure: insecure,

			MaxConns:     concurrency,
			DoTimeout:    timeout,
			ReadTimeout:  respReadTimeout,
			WriteTimeout: reqWriteTimeout,
			DialTimeout:  dialTimeout,

			Socks5Proxy: socks5,
			ContentType: contentType,
			Host:        host,
		}

		requester, err := req.NewRequester(concurrency, requests, duration, &clientOpt)
		if err != nil {
			errAndExit(err.Error())
			return
		}

		outStream := os.Stdout
		if summary {
			outStream = os.Stderr
			// isTerminal = false
		}
		// description
		var desc string
		desc = fmt.Sprintf("Benchmarking %s", url)
		if requests > 0 {
			desc += fmt.Sprintf(" with %d request(s)", requests)
		}
		if duration > 0 {
			desc += fmt.Sprintf(" for %s", duration.String())
		}
		desc += fmt.Sprintf(" using %d connection(s).", concurrency)
		fmt.Fprintln(outStream, desc)

		fmt.Fprintln(outStream, "")

		// do request
		go requester.Run()

		// metrics collection
		report := req.NewStreamReport()
		go report.Collect(requester.RecordChan())

		// terminal printer
		printer := req.NewPrinter(requests, duration, !clean, summary)
		printer.PrintLoop(report.Snapshot, interval, seconds, report.Done())

	},
}

func init() {
	rootCmd.AddCommand(httpbenchmarkCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// httpbenchmarkCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// httpbenchmarkCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	httpbenchmarkCmd.Flags().IntP("concurrency", "c", 1, "Number of connections to run concurrently")
	httpbenchmarkCmd.Flags().Int64P("requests", "n", -1, "Number of requests to run")
	duration, _ := time.ParseDuration("1m")
	httpbenchmarkCmd.Flags().DurationP("duration", "d", duration, "Duration of test, examples: -d 10s -d 3m")
	interval, _ := time.ParseDuration("200ms")
	httpbenchmarkCmd.Flags().DurationP("interval", "i", interval, "Print snapshot result every interval, use 0 to print once at the end")
	httpbenchmarkCmd.Flags().Bool("seconds", true, "Use seconds as time unit to print")

	httpbenchmarkCmd.Flags().StringP("body", "b", "", "HTTP request body, if start the body with @, the rest should be a filename to read")
	httpbenchmarkCmd.Flags().Bool("stream", false, "Specify whether to stream file specified by '--body @file' using chunked encoding or to read into memory")
	httpbenchmarkCmd.Flags().StringP("method", "m", "GET", "HTTP method")
	httpbenchmarkCmd.Flags().StringSliceP("header", "H", []string{}, "Custom HTTP headers")
	httpbenchmarkCmd.Flags().String("host", "", "Host header")
	httpbenchmarkCmd.Flags().StringP("content", "T", "", "Content-Type header")
	httpbenchmarkCmd.Flags().String("cert", "", "Path to the client's TLS Certificate")
	httpbenchmarkCmd.Flags().String("key", "", "Path to the client's TLS Certificate Private Key")
	httpbenchmarkCmd.Flags().BoolP("insecure", "k", true, "Controls whether a client verifies the server's certificate chain and host name")

	timeout, _ := time.ParseDuration("1m")
	httpbenchmarkCmd.Flags().Duration("timeout", timeout, "Timeout for each http request")
	httpbenchmarkCmd.Flags().Duration("dial-timeout", timeout, "Timeout for dial addr")
	httpbenchmarkCmd.Flags().Duration("req-timeout", timeout, "Timeout for full request writing")
	httpbenchmarkCmd.Flags().Duration("resp-timeout", timeout, "Timeout for full response reading")
	httpbenchmarkCmd.Flags().String("socks5", "", "Socks5 proxy [ip:port]")

	httpbenchmarkCmd.Flags().Bool("clean", true, "Clean the histogram bar once its finished. Default is true")
	httpbenchmarkCmd.Flags().Bool("summary", false, "Only print the summary without realtime reports")
	// httpbenchmarkCmd.Arg("url", "request url").Required().String()

}
