package cmd

import (
	"fmt"
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
	Short: "gpgenie 是一个用于管理和分析 PGP 密钥的命令行工具",
	Long:  `gpgenie 可以生成、展示、导出和分析 PGP 密钥，帮助用户管理密钥信息。`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// 初始化应用
		appInstance, err := app.NewApp(cfgFile)
		if err != nil {
			log.Errorf("初始化应用失败: %v", err)
			os.Exit(1)
		}

		log.Infof("使用配置文件: %s", viper.ConfigFileUsed())
		viper.Set("app", appInstance)
	},
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		log.Errorf("执行命令失败: %v", err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "config/config.json", "配置文件路径")

	if err := viper.BindPFlag("config", RootCmd.PersistentFlags().Lookup("config")); err != nil {
		fmt.Printf("绑定配置文件标志失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	var err error
	log, err = logger.InitLogger(&config.LoggingConfig{})
	if err != nil {
		fmt.Printf("初始化日志失败: %v\n", err)
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
		log.Errorf("读取配置文件失败: %v", err)
	}
}
