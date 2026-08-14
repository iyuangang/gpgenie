package cmd

import (
	"fmt"

	"github.com/iyuangang/gpgenie/internal/app"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var AnalyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "analyze key data",
	Long:  `Analyze PGP key data in the database, including scoring statistics and correlation analysis.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		appInterface := viper.Get("app")
		appInstance, ok := appInterface.(*app.App)
		if !ok {
			return fmt.Errorf("failed to get app instance")
		}

		if err := appInstance.KeyService.AnalyzeData(); err != nil {
			return fmt.Errorf("analyze key data: %w", err)
		}

		log.Info("key data analysis completed.")
		return nil
	},
}

func init() {
	RootCmd.AddCommand(AnalyzeCmd)
}
