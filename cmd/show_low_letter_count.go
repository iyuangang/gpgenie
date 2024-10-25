package cmd

import (
	"fmt"

	"gpgenie/internal/app"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var showLowLetterCountN int

var ShowLowLetterCountCmd = &cobra.Command{
	Use:   "show-low-letter-count",
	Short: "显示唯一字母数量最少的密钥",
	Long:  `显示数据库中唯一字母数量最少的 N 个 PGP 密钥。`,
	Run: func(cmd *cobra.Command, args []string) {
		appInterface := viper.Get("app")
		appInstance, ok := appInterface.(*app.App)
		if !ok {
			fmt.Println("无法获取应用实例")
			return
		}

		if err := appInstance.KeyService.ShowLowLetterCountKeys(showLowLetterCountN); err != nil {
			fmt.Printf("显示低唯一字母数密钥失败: %v\n", err)
			return
		}
	},
}

func init() {
	RootCmd.AddCommand(ShowLowLetterCountCmd)

	ShowLowLetterCountCmd.Flags().IntVarP(&showLowLetterCountN, "n", "n", 10, "显示的密钥数量")
}
