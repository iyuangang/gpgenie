package cmd

import (
	"os"

	"gpgenie/internal/app"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	exportFingerprint string
	exportOutputDir   string
	exportArmor       bool
)

var ExportCmd = &cobra.Command{
	Use:   "export",
	Short: "export key by fingerprint",
	Long:  `Export PGP keys by fingerprint to the specified directory.`,
	Run: func(cmd *cobra.Command, args []string) {
		appInterface := viper.Get("app")
		appInstance, ok := appInterface.(*app.App)
		if !ok {
			log.Error("failed to get app instance")
			return
		}

		if err := appInstance.KeyService.ExportKeyByFingerprint(exportFingerprint, exportOutputDir, exportArmor); err != nil {
			log.Errorf("failed to export key: %v", err)
			return
		}

		log.Info("key exported successfully.")
	},
}

func init() {
	RootCmd.AddCommand(ExportCmd)

	ExportCmd.Flags().StringVarP(&exportFingerprint, "fingerprint", "f", "", "the last 16 digits of the fingerprint (required)")
	err := ExportCmd.MarkFlagRequired("fingerprint")
	if err != nil {
		log.Errorf("failed to set fingerprint flag: %v", err)
		os.Exit(1)
	}
	ExportCmd.Flags().StringVarP(&exportOutputDir, "output-dir", "o", "./exported_keys", "the directory to export keys")
	ExportCmd.Flags().BoolVarP(&exportArmor, "armor", "a", true, "whether to use ASCII Armor to export private keys")
}
