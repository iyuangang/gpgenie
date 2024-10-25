package cmd

import (
	"fmt"
	"os"

	"gpgenie/internal/app"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var RootCmd = &cobra.Command{
	Use:   "gpgenie",
	Short: "gpgenie 是一个用于管理和分析 PGP 密钥的命令行工具",
	Long:  `gpgenie 可以生成、展示、导出和分析 PGP 密钥，帮助用户管理密钥信息。`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// 初始化应用
		appInstance, err := app.NewApp(cfgFile)
		if err != nil {
			fmt.Printf("初始化应用失败: %v\n", err)
			os.Exit(1)
		}

		// 将 appInstance 存储在 Viper 中，以便子命令访问
		viper.Set("app", appInstance)
	},
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// 定义全局配置文件标志
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "config/config.json", "配置文件路径 (默认为 config/config.json)")

	// 绑定配置文件标志到 Viper
	err := viper.BindPFlag("config", RootCmd.PersistentFlags().Lookup("config"))
	if err != nil {
		fmt.Printf("绑定配置文件标志失败: %v\n", err)
		os.Exit(1)
	}
}

func initConfig() {
	if cfgFile != "" {
		// 指定配置文件
		viper.SetConfigFile(cfgFile)
	} else {
		// 默认配置文件路径
		viper.AddConfigPath(".")
		viper.SetConfigName("config")
	}

	// 读取配置文件
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("使用配置文件:", viper.ConfigFileUsed())
	}
}
