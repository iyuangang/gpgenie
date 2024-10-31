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
	Short: "generate PGP key pairs",
	Long:  `Generate a specified number of PGP key pairs according to the configuration and store them in the database.`,
	Run: func(cmd *cobra.Command, args []string) {
		appInterface := viper.Get("app")
		appInstance, ok := appInterface.(*app.App)
		if !ok {
			log.Error("failed to get app instance")
			return
		}

		// check if the command line parameters are explicitly set
		totalFlagChanged := cmd.Flags().Changed("total")
		batchFlagChanged := cmd.Flags().Changed("batch")

		// if not specified, use the value from the config file
		if !totalFlagChanged {
			totalKeys = appInstance.Config.KeyGeneration.TotalKeys
		}
		if !batchFlagChanged {
			batchSize = appInstance.Config.KeyGeneration.BatchSize
		}

		log.Infof("start generating keys, total: %d, batch size: %d", totalKeys, batchSize)

		// update config
		appInstance.Config.KeyGeneration.TotalKeys = totalKeys
		appInstance.Config.KeyGeneration.BatchSize = batchSize

		if err := appInstance.KeyService.GenerateKeys(context.Background()); err != nil {
			log.Errorf("failed to generate keys: %v", err)
			return
		}

		log.Info("keys generated successfully.")
	},
}

func init() {
	RootCmd.AddCommand(GenerateCmd)

	// 设置默认值为0，这样可以判断是否使用配置文件的值
	GenerateCmd.Flags().IntVarP(&totalKeys, "total", "t", 0, "the total number of keys to generate (default from config if not specified)")
	GenerateCmd.Flags().IntVarP(&batchSize, "batch", "b", 0, "the number of keys to insert in batches (default from config if not specified)")
}
