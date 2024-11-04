package cmd

import (
	"os"

	"gpgenie/internal/app"
	"gpgenie/internal/config"
	"gpgenie/internal/logger"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	log     *logger.Logger
)

var RootCmd = &cobra.Command{
	Use:   "gpgenie",
	Short: "gpgenie is a command-line tool for managing and analyzing PGP keys",
	Long:  `gpgenie can generate, display, export, and analyze PGP keys, helping users manage key information.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// initialize app
		appInstance, err := app.NewApp(cfgFile)
		if err != nil {
			log.Errorf("failed to initialize app: %v", err)
			os.Exit(1)
		}

		log.Debugf("using config file: %s", viper.ConfigFileUsed())
		viper.Set("app", appInstance)
	},
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		log.Errorf("failed to execute command: %v", err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "config/config.json", "config file path")

	if err := viper.BindPFlag("config", RootCmd.PersistentFlags().Lookup("config")); err != nil {
		log.Errorf("failed to bind config flag: %v", err)
		os.Exit(1)
	}

	// initialize logger
	var err error
	log, err = logger.InitLogger(&config.LoggingConfig{})
	if err != nil {
		log.Errorf("failed to initialize logger: %v", err)
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
