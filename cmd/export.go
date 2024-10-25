package cmd

import (
	"fmt"
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
	Short: "导出指定指纹的密钥",
	Long:  `根据提供的指纹，导出相应的 PGP 密钥到指定目录。`,
	Run: func(cmd *cobra.Command, args []string) {
		appInterface := viper.Get("app")
		appInstance, ok := appInterface.(*app.App)
		if !ok {
			fmt.Println("无法获取应用实例")
			return
		}

		if err := appInstance.KeyService.ExportKeyByFingerprint(exportFingerprint, exportOutputDir, exportArmor); err != nil {
			fmt.Printf("导出密钥失败: %v\n", err)
			return
		}

		fmt.Println("密钥导出成功。")
	},
}

func init() {
	RootCmd.AddCommand(ExportCmd)

	ExportCmd.Flags().StringVarP(&exportFingerprint, "fingerprint", "f", "", "密钥的最后十六位指纹 (必需)")
	err := ExportCmd.MarkFlagRequired("fingerprint")
	if err != nil {
		fmt.Printf("设置指纹标志失败: %v\n", err)
		os.Exit(1)
	}
	ExportCmd.Flags().StringVarP(&exportOutputDir, "output-dir", "o", "./exported_keys", "密钥导出目录")
	ExportCmd.Flags().BoolVarP(&exportArmor, "armor", "a", true, "是否使用 ASCII Armor 导出私钥")
}
