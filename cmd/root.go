package cmd

import (
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gubal",
	Short: "A scraper/API for public FFXIV data",
	Long:  `A scraper/API for public FFXIV data.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		lconf := zap.NewDevelopmentConfig()
		if viper.GetBool("prod") {
			lconf = zap.NewProductionConfig()
		}
		l, err := lconf.Build()
		if err != nil {
			return err
		}
		zap.ReplaceGlobals(l)
		zap.RedirectStdLog(l)
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().BoolP("prod", "P", false, "run in production mode")
	rootCmd.PersistentFlags().String("nsqd", "127.0.0.1:4150", "nsqd instance for publishing")
	rootCmd.PersistentFlags().String("nsqlookupd", "127.0.0.1:4161", "nsqlookupd instance for consumption")
	rootCmd.PersistentFlags().StringP("db", "d", "postgres:///gubal?sslmode=disable", "database connection string")
	must(viper.BindPFlags(rootCmd.PersistentFlags()))
}

func initConfig() {
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()
}
