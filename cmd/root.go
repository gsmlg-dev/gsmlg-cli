/*
Copyright © 2022 Jonathan Gao gsmlg.com@gmail.com

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gsmlg-dev/gsmlg-golang/errorhandler"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const defualtFname = ".config/gsmlg/cli.yaml"

var (
	cfgFile string
	Version string = "dev"
)

var exitIfError = errorhandler.CreateExitIfError("GSMLG CLI Error")

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gsmlg-cli",
	Short: "A command line tool for my private affair.",
	Long:  `A command line tool for my private affair.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		v, _ := cmd.Flags().GetBool("version")
		if v {
			fmt.Printf("Version: %s\n", Version)
			os.Exit(0)
		}
		cmd.Help()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/"+defualtFname+")")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("version", "", false, "Print version")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".cli" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		fName := defualtFname
		viper.SetConfigName(fName)
		cfgFile = filepath.Join(home, fName)
		viper.SetConfigFile(cfgFile)
	}

	if _, err := os.Stat(cfgFile); err != nil {
		fmt.Printf("Cofnig file %s not exists, please create it.\n", cfgFile)
		// f, err := os.Create(cfgFile)
		// cobra.CheckErr(err)
		// defer f.Close()
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	// if err := viper.ReadInConfig(); err == nil {
	// fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	// }
}

func writeConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".cli" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		fName := defualtFname
		viper.SetConfigName(fName)
		cfgFile = filepath.Join(home, fName)
		viper.SetConfigFile(cfgFile)
	}

	err := viper.WriteConfig()
	cobra.CheckErr(err)
}
