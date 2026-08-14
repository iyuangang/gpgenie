package cmd

import (
	"fmt"
	"os"

	"github.com/iyuangang/gpgenie/internal/app"

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
	RunE: func(cmd *cobra.Command, args []string) error {
		appInterface := viper.Get("app")
		appInstance, ok := appInterface.(*app.App)
		if !ok {
			return fmt.Errorf("failed to get app instance")
		}

		if err := appInstance.KeyService.ExportKeyByFingerprint(exportFingerprint, exportOutputDir, exportArmor); err != nil {
			return fmt.Errorf("export key: %w", err)
		}

		log.Info("key exported successfully.")
		return nil
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
