package cmd

import (
	"fmt"

	"gpgenie/internal/app"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var AnalyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "分析密钥数据",
	Long:  `对数据库中的 PGP 密钥数据进行统计分析，包括评分统计和相关性分析。`,
	Run: func(cmd *cobra.Command, args []string) {
		appInterface := viper.Get("app")
		appInstance, ok := appInterface.(*app.App)
		if !ok {
			fmt.Println("无法获取应用实例")
			return
		}

		if err := appInstance.KeyService.AnalyzeData(); err != nil {
			fmt.Printf("分析密钥数据失败: %v\n", err)
			return
		}

		fmt.Println("密钥数据分析完成。")
	},
}

func init() {
	RootCmd.AddCommand(AnalyzeCmd)
}
