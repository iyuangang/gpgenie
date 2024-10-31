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

		log.Infof("start generating keys, total: %d, batch size: %d", totalKeys, batchSize)

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

	GenerateCmd.Flags().IntVarP(&totalKeys, "total", "t", 100, "the total number of keys to generate")
	GenerateCmd.Flags().IntVarP(&batchSize, "batch", "b", 10, "the number of keys to insert in batches")
}
