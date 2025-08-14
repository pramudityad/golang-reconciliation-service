package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	verbose bool
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "reconciler",
	Short: "Transaction reconciliation tool",
	Long: `Reconciler is a command-line tool for reconciling system transactions
with bank statements. It supports multiple bank formats, flexible matching
algorithms, and comprehensive reporting.

Examples:
  reconciler reconcile --system-file transactions.csv --bank-files statements.csv
  reconciler reconcile --system-file tx.csv --bank-files bank1.csv,bank2.csv --output-format json
  reconciler version`,
	Version: getVersionString(),
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (optional)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Bind flags to viper
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
}

// initConfig reads in config file and ENV variables.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
		
		// If a config file is specified, read it in.
		if err := viper.ReadInConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Error reading config file: %s\n", err)
			os.Exit(1)
		}
		
		if viper.GetBool("verbose") {
			fmt.Fprintf(os.Stderr, "Using config file: %s\n", viper.ConfigFileUsed())
		}
	}

	// Read environment variables that match
	viper.SetEnvPrefix("RECONCILER")
	viper.AutomaticEnv()
}

// SetVersionInfo sets the version information for the CLI
func SetVersionInfo(v, c, d string) {
	version = v
	commit = c
	date = d
	rootCmd.Version = getVersionString()
}

func getVersionString() string {
	if version == "dev" {
		return fmt.Sprintf("%s (commit %s, built %s)", version, commit, date)
	}
	return version
}
