package commands

import (
	"fmt"
	"github.com/spf13/cobra"
	jww "github.com/spf13/jwalterweatherman"
	"github.com/spf13/viper"
)

const mainDescription = `go-blc is a tool that finds broken links in websites.

It will crawl a website and report all links that don't return a successful HTTP status code.`

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var versionTemplate = fmt.Sprintf(`Version: %s
Commit: %s
Built: %s`, version, commit, date+"\n")

var verbose bool

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "enable verbose output")
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
}

func initConfig() {
	viper.AddConfigPath(".")
	viper.SetConfigName("goblc")

	if err := viper.ReadInConfig(); err != nil {
		jww.WARN.Printf("error reading config, using defaults")
	}

	viper.SetEnvPrefix("goblc")

	viper.SetDefault("verbose", false)
}

var rootCmd = &cobra.Command{
	Use:     "chrono",
	Long:    mainDescription,
	Version: version,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if viper.GetBool("verbose") {
			jww.SetLogThreshold(jww.LevelDebug)
			jww.SetStdoutThreshold(jww.LevelDebug)
		}
	},
}

// Execute creates the root command with all sub-commands installed
func Execute() {
	jww.SetLogThreshold(jww.LevelError)
	jww.SetStdoutThreshold(jww.LevelError)
	rootCmd.SetVersionTemplate(versionTemplate)
	rootCmd.AddCommand(
		newScan(),
	)
	rootCmd.Execute()
}
