package cmd

import (
	"gpgenie/internal/app"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var displayCount int // the unified display count parameter

// ShowCmd the main command to display key information
var ShowCmd = &cobra.Command{
	Use:   "show",
	Short: "display key information",
	Long:  `display key information in the database, support sorting and filtering by different conditions.`,
}

// ShowTopCmd the subcommand to display high-score keys
var ShowTopCmd = &cobra.Command{
	Use:   "top",
	Short: "display the highest-scoring keys",
	Long:  `display the highest N PGP keys in the database.`,
	Run: func(cmd *cobra.Command, args []string) {
		appInterface := viper.Get("app")
		appInstance, ok := appInterface.(*app.App)
		if !ok {
			log.Error("failed to get app instance")
			return
		}

		log.Debugf("display the highest %d keys", displayCount)
		if err := appInstance.KeyService.ShowTopKeys(displayCount); err != nil {
			log.Errorf("failed to display high-scoring keys: %v", err)
			return
		}
	},
}

// ShowMinimalKeysCmd the subcommand to display minimal keys
var ShowMinimalKeysCmd = &cobra.Command{
	Use:   "minimal",
	Short: "display minimal keys",
	Long:  `display the N PGP keys with the fewest characters in the database.`,
	Run: func(cmd *cobra.Command, args []string) {
		appInterface := viper.Get("app")
		appInstance, ok := appInterface.(*app.App)
		if !ok {
			log.Error("failed to get app instance")
			return
		}

		log.Debugf("display the minimal %d keys", displayCount)
		if err := appInstance.KeyService.ShowMinimalKeys(displayCount); err != nil {
			log.Errorf("failed to display minimal keys: %v", err)
			return
		}
	},
}

func init() {
	// add show command to root command
	RootCmd.AddCommand(ShowCmd)

	// add subcommands to show command
	ShowCmd.AddCommand(ShowTopCmd)
	ShowCmd.AddCommand(ShowMinimalKeysCmd)

	// add common -n flag to subcommands
	ShowTopCmd.Flags().IntVarP(&displayCount, "count", "n", 10, "the number of keys to display")
	ShowMinimalKeysCmd.Flags().IntVarP(&displayCount, "count", "n", 10, "the number of keys to display")
}
