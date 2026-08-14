package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/iyuangang/gpgenie/internal/app"
	"github.com/iyuangang/gpgenie/internal/config"
	"github.com/iyuangang/gpgenie/internal/logger"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	log     *logger.Logger
)

var RootCmd = &cobra.Command{
	Use:           "gpgenie",
	Short:         "gpgenie is a command-line tool for managing and analyzing PGP keys",
	Long:          `gpgenie can generate, display, export, and analyze PGP keys, helping users manage key information.`,
	SilenceErrors: true,
	SilenceUsage:  true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// initialize app
		appInstance, err := app.NewApp(cfgFile)
		if err != nil {
			return fmt.Errorf("initialize app: %w", err)
		}

		log.Debugf("using config file: %s", viper.ConfigFileUsed())
		viper.Set("app", appInstance)
		return nil
	},
}

func Execute(ctx context.Context) {
	err := RootCmd.ExecuteContext(ctx)
	if err != nil {
		log.Errorf("failed to execute command: %v", err)
	}

	if appInstance, ok := viper.Get("app").(*app.App); ok {
		if closeErr := appInstance.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "failed to close app: %v\n", closeErr)
		}
	}
	log.SyncLogger()

	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "config/config.json", "config file path")

	if err := viper.BindPFlag("config", RootCmd.PersistentFlags().Lookup("config")); err != nil {
		fmt.Fprintf(os.Stderr, "failed to bind config flag: %v\n", err)
		os.Exit(1)
	}

	// initialize logger
	var err error
	log, err = logger.InitLogger(&config.LoggingConfig{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath(".")
		viper.SetConfigName("config")
	}

	if err := viper.ReadInConfig(); err != nil {
		log.Errorf("failed to read config file: %v", err)
	}
}
