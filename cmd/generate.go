package cmd

import (
	"context"

	"gpgenie/internal/app"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	totalKeys int
	batchSize int
)

var GenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "生成 PGP 密钥对",
	Long:  `根据配置生成指定数量的 PGP 密钥对，并将其存储到数据库中。`,
	Run: func(cmd *cobra.Command, args []string) {
		appInterface := viper.Get("app")
		appInstance, ok := appInterface.(*app.App)
		if !ok {
			log.Error("无法获取应用实例")
			return
		}

		log.Infof("开始生成密钥，总数: %d, 批次大小: %d", totalKeys, batchSize)

		appInstance.Config.KeyGeneration.TotalKeys = totalKeys
		appInstance.Config.KeyGeneration.BatchSize = batchSize

		if err := appInstance.KeyService.GenerateKeys(context.Background()); err != nil {
			log.Errorf("生成密钥失败: %v", err)
			return
		}

		log.Info("密钥生成完成")
	},
}

func init() {
	RootCmd.AddCommand(GenerateCmd)

	GenerateCmd.Flags().IntVarP(&totalKeys, "total", "t", 100, "生成的密钥总数")
	GenerateCmd.Flags().IntVarP(&batchSize, "batch", "b", 10, "批量插入的密钥数")
}
