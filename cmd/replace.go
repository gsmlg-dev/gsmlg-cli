/*
Copyright © 2024 Jonathan Gao <gsmlg.com@gmail.com>
*/
package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// replaceCmd replace strings
var replaceCmd = &cobra.Command{
	Use:   "replace",
	Short: "replace string in directory.",
	Long: `replace string in directory.
  gsmlg-cli replace --from "<name1>" --to "<name2>" --only-in <dir?>.`,
	Run: func(cmd *cobra.Command, args []string) {
		from, err := cmd.Flags().GetString("from")
		exitIfError(err)
		to, err := cmd.Flags().GetString("to")
		exitIfError(err)
		onlyIn, err := cmd.Flags().GetString("only-in")
		exitIfError(err)
		fmt.Printf("from: %s, to: %s, in: %s\n", from, to, onlyIn)
		if from != "" && to != "" {
			if onlyIn != "" {
				// replace in only in dir
				matches, err := findFiles(onlyIn)
				exitIfError(err)
				for _, f := range matches {
					e := replaceStringInFile(f, from, to)
					if e != nil {
						fmt.Fprintf(os.Stderr, "%s", e)
					}
				}
			} else {
				// replace in cwd
				currentDir, err := os.Getwd()
				exitIfError(err)
				matches, err := findFiles(currentDir)
				exitIfError(err)
				for _, f := range matches {
					e := replaceStringInFile(f, from, to)
					if e != nil {
						fmt.Fprintf(os.Stderr, "%s", e)
					}
				}
			}
		} else if to == "" && from != "" {
			// replace to is empty, do search only
			if onlyIn != "" {
				// search in only in dir
				matches, err := findFiles(onlyIn)
				exitIfError(err)
				for _, f := range matches {
					e := findStringInFile(f, from)
					if e != nil {
						fmt.Fprintf(os.Stderr, "%s", e)
					}
				}
			} else {
				// replace in cwd
				currentDir, err := os.Getwd()
				exitIfError(err)
				matches, err := findFiles(currentDir)
				exitIfError(err)
				for _, f := range matches {
					e := findStringInFile(f, from)
					if e != nil {
						fmt.Fprintf(os.Stderr, "%s", e)
					}
				}
			}
		} else {
			// print files
			if onlyIn != "" {
				matches, err := findFiles(onlyIn)
				exitIfError(err)
				for _, f := range matches {
					fmt.Println(f)
				}
			} else {
				currentDir, err := os.Getwd()
				exitIfError(err)
				matches, err := findFiles(currentDir)
				exitIfError(err)
				for _, f := range matches {
					fmt.Println(f)
				}
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(replaceCmd)

	replaceCmd.Flags().StringP("from", "f", "", "replace from")
	replaceCmd.Flags().StringP("to", "t", "", "replace to")
	replaceCmd.Flags().StringP("only-in", "d", "", "replace only in directory")
}

func replaceStringInFile(filePath string, oldString, newString string) error {
	fmt.Printf("Start replace in file: %s\n", filePath)
	// Open the file for reading and writing
	file, err := os.OpenFile(filePath, os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Read the entire file content into a byte slice
	data, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Replace occurrences of oldString with newString in the byte slice
	newData := bytes.ReplaceAll(data, []byte(oldString), []byte(newString))

	// Truncate the file to clear its contents
	err = file.Truncate(0)
	if err != nil {
		return fmt.Errorf("failed to truncate file: %w", err)
	}

	// Seek to the beginning of the file
	_, err = file.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("failed to seek file: %w", err)
	}

	// Write the modified data back to the file
	_, err = file.Write(newData)
	if err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	return nil
}

func findStringInFile(filePath string, matchString string) error {
	fmt.Printf("Start find in file: %s\n", filePath)

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNumber := 1

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, matchString) {
			fmt.Printf("Line %d: %s\n", lineNumber, line)
		}
		lineNumber++
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error scanning file: %w", err)
	}

	return nil
}

func findFiles(arg string) ([]string, error) {
	var foundFiles []string

	if !filepath.IsAbs(arg) {
		currentDir, err := os.Getwd()
		exitIfError(err)
		arg = filepath.Join(currentDir, arg)
	}

	// Check if arg is a path with a pattern or just a directory path
	if strings.ContainsRune(arg, '*') || strings.ContainsRune(arg, '?') {
		// Treat arg as a path with a pattern
		rootPath, pattern, err := parsePathPattern(arg)
		if err != nil {
			return nil, fmt.Errorf("invalid path or pattern: %w", err)
		}
		fmt.Printf("Warking in %s with pattern %s\n", rootPath, pattern)
		err = filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
					foundFiles = append(foundFiles, path)
				}
			}
			return nil
		})
		return foundFiles, err
	} else {
		// Treat arg as a directory path
		rootPath := arg
		fmt.Printf("Warking in %s\n", rootPath)
		err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				foundFiles = append(foundFiles, path)
			}
			return nil
		})
		return foundFiles, err
	}
}

// Helper function to parse path and pattern from a combined argument
func parsePathPattern(arg string) (string, string, error) {

	lastSlash := strings.LastIndex(arg, string(filepath.Separator))
	if lastSlash == -1 {
		return arg, "", nil
	}
	rootPath := arg[:lastSlash]
	pattern := arg[lastSlash+1:]
	return rootPath, pattern, nil
}
