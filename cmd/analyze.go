package cmd

import (
	"gpgenie/internal/app"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var AnalyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "analyze key data",
	Long:  `Analyze PGP key data in the database, including scoring statistics and correlation analysis.`,
	Run: func(cmd *cobra.Command, args []string) {
		appInterface := viper.Get("app")
		appInstance, ok := appInterface.(*app.App)
		if !ok {
			log.Error("failed to get app instance")
			return
		}

		if err := appInstance.KeyService.AnalyzeData(); err != nil {
			log.Errorf("failed to analyze key data: %v", err)
			return
		}

		log.Info("key data analysis completed.")
	},
}

func init() {
	RootCmd.AddCommand(AnalyzeCmd)
}
