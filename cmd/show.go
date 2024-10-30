package cmd

import (
	"gpgenie/internal/app"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var displayCount int // 统一的显示数量参数

// ShowCmd 展示密钥信息的主命令
var ShowCmd = &cobra.Command{
	Use:   "show",
	Short: "显示密钥信息",
	Long:  `显示数据库中的密钥信息，支持按不同条件排序和筛选。`,
}

// ShowTopCmd 显示高分密钥的子命令
var ShowTopCmd = &cobra.Command{
	Use:   "top",
	Short: "显示评分最高的密钥",
	Long:  `显示数据库中评分最高的 N 个 PGP 密钥。`,
	Run: func(cmd *cobra.Command, args []string) {
		appInterface := viper.Get("app")
		appInstance, ok := appInterface.(*app.App)
		if !ok {
			log.Error("无法获取应用实例")
			return
		}

		log.Debugf("显示评分最高的 %d 个密钥", displayCount)
		if err := appInstance.KeyService.ShowTopKeys(displayCount); err != nil {
			log.Errorf("显示高分密钥失败: %v", err)
			return
		}
	},
}

// ShowMinimalKeysCmd 显示最简密钥的子命令
var ShowMinimalKeysCmd = &cobra.Command{
	Use:   "minimal",
	Short: "显示最简密钥",
	Long:  `显示数据库中字符种类最少的 N 个 PGP 密钥。`,
	Run: func(cmd *cobra.Command, args []string) {
		appInterface := viper.Get("app")
		appInstance, ok := appInterface.(*app.App)
		if !ok {
			log.Error("无法获取应用实例")
			return
		}

		log.Debugf("显示字符种类最少的 %d 个密钥", displayCount)
		if err := appInstance.KeyService.ShowMinimalKeys(displayCount); err != nil {
			log.Errorf("显示最简密钥失败: %v", err)
			return
		}
	},
}

func init() {
	// 添加 show 命令到根命令
	RootCmd.AddCommand(ShowCmd)

	// 添加子命令到 show 命令
	ShowCmd.AddCommand(ShowTopCmd)
	ShowCmd.AddCommand(ShowMinimalKeysCmd)

	// 为子命令添加共同的 -n 标志
	ShowTopCmd.Flags().IntVarP(&displayCount, "count", "n", 10, "显示的密钥数量")
	ShowMinimalKeysCmd.Flags().IntVarP(&displayCount, "count", "n", 10, "显示的密钥数量")
}
