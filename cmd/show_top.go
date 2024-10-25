package cmd

import (
	"fmt"

	"gpgenie/internal/app"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var showTopN int

var ShowTopCmd = &cobra.Command{
	Use:   "show-top",
	Short: "显示评分最高的密钥",
	Long:  `显示数据库中评分最高的 N 个 PGP 密钥。`,
	Run: func(cmd *cobra.Command, args []string) {
		appInterface := viper.Get("app")
		appInstance, ok := appInterface.(*app.App)
		if !ok {
			fmt.Println("无法获取应用实例")
			return
		}

		if err := appInstance.KeyService.ShowTopKeys(showTopN); err != nil {
			fmt.Printf("显示高分密钥失败: %v\n", err)
			return
		}
	},
}

func init() {
	RootCmd.AddCommand(ShowTopCmd)

	ShowTopCmd.Flags().IntVarP(&showTopN, "n", "n", 10, "显示的密钥数量")
}
