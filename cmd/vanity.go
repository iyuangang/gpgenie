package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"time"

	"github.com/iyuangang/gpgenie/internal/app"
	"github.com/iyuangang/gpgenie/internal/key/service"
	"github.com/iyuangang/gpgenie/internal/key/vanity"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	vanityMinRun           int
	vanityWorkers          int
	vanityScope            string
	vanityDigits           string
	vanityMaxAttempts      uint64
	vanityTimestampWindow  time.Duration
	vanityOutputDir        string
	vanityCheckpointPath   string
	vanityResume           bool
	vanityProgressInterval time.Duration
)

var VanityCmd = &cobra.Command{
	Use:   "vanity",
	Short: "mine a real OpenPGP vanity signing subkey",
	Long: `Mine an Ed25519 OpenPGP signing subkey whose 16-digit long key ID
contains a repeated hexadecimal run. The normal primary key is generated only
after a result is found and the private keyring is encrypted to the configured
encryptor_public_key.`,
	RunE: runVanity,
}

func runVanity(cmd *cobra.Command, _ []string) error {
	appInterface := viper.Get("app")
	appInstance, ok := appInterface.(*app.App)
	if !ok {
		return fmt.Errorf("failed to get app instance")
	}

	workers := vanityWorkers
	if workers == 0 {
		workers = appInstance.Config.KeyGeneration.NumGeneratorWorkers
	}
	if workers == 0 {
		workers = runtime.NumCPU()
	}
	if vanityTimestampWindow < time.Second {
		return fmt.Errorf("timestamp-window must be at least one second")
	}
	scope := vanity.Scope(vanityScope)
	if err := scope.Validate(); err != nil {
		return err
	}
	allowedDigits, err := vanity.ParseDigits(vanityDigits)
	if err != nil {
		return fmt.Errorf("invalid --digits: %w", err)
	}
	targetDigits := allowedDigits.String()

	now := time.Now().UTC().Truncate(time.Second)
	start := now.Add(-vanityTimestampWindow)
	if start.Unix() < 1 || now.Unix() > int64(^uint32(0)) {
		return fmt.Errorf("timestamp window is outside the OpenPGP v4 timestamp range")
	}
	primaryCreatedAt := start.Add(-time.Second)

	checkpointPath := vanityCheckpointPath
	if checkpointPath == "" {
		checkpointPath = filepath.Join(vanityOutputDir, "vanity-checkpoint.json")
	}
	checkpoint := &vanity.Checkpoint{}
	if vanityResume {
		loaded, err := vanity.LoadCheckpoint(checkpointPath)
		if err != nil {
			return err
		}
		checkpoint = loaded
	}
	if checkpointHasSearchState(checkpoint) {
		checkpointScope := checkpoint.Scope
		if checkpointScope == "" {
			checkpointScope = vanity.ScopeSuffix
		}
		checkpointDigits := checkpoint.TargetDigits
		if checkpointDigits == "" {
			checkpointDigits = vanity.AllDigits.String()
		} else if parsed, parseErr := vanity.ParseDigits(checkpointDigits); parseErr == nil {
			checkpointDigits = parsed.String()
		}
		if checkpointScope != scope || checkpointDigits != targetDigits {
			fmt.Fprintf(
				cmd.OutOrStdout(),
				"checkpoint criteria changed: scope=%s digits=%s -> scope=%s digits=%s; resetting counters and best (existing artifacts are preserved)\n",
				checkpointScope, checkpointDigits, scope, targetDigits,
			)
			checkpoint = &vanity.Checkpoint{}
		}
	}
	checkpoint.Scope = scope
	checkpoint.TargetDigits = targetDigits
	if checkpoint.BestRun >= vanityMinRun {
		fmt.Fprintf(cmd.OutOrStdout(), "checkpoint already contains run=%d key_id=%s\n", checkpoint.BestRun, checkpoint.BestKeyID)
		return nil
	}

	fmt.Fprintf(
		cmd.OutOrStdout(),
		"vanity search started: workers=%d scope=%s digits=%s target_run=%d timestamp_window=%s previous_attempts=%d\n",
		workers, scope, targetDigits, vanityMinRun, vanityTimestampWindow, checkpoint.Attempts,
	)
	fmt.Fprintf(
		cmd.OutOrStdout(),
		"key timestamps will span %s through %s; each additional repeated digit costs about 16x more work\n",
		start.Format(time.RFC3339), now.Format(time.RFC3339),
	)

	searchConfig := vanity.SearchConfig{
		Workers:          workers,
		MinRun:           vanityMinRun,
		Scope:            scope,
		AllowedDigits:    allowedDigits,
		TimestampStart:   uint32(start.Unix()),
		TimestampEnd:     uint32(now.Unix()),
		MaxAttempts:      vanityMaxAttempts,
		InitialAttempts:  checkpoint.Attempts,
		InitialBestRun:   checkpoint.BestRun,
		ProgressInterval: vanityProgressInterval,
	}

	var checkpointErr error
	result, searchErr := vanity.Search(cmd.Context(), searchConfig, func(progress vanity.Progress) {
		checkpoint.Attempts = progress.Attempts
		// Only attempts are durable while mining. A promoted candidate's
		// private material exists in memory until finalization succeeds, so do
		// not replace the durable best checkpoint prematurely.
		if err := vanity.SaveCheckpoint(checkpointPath, *checkpoint); err != nil && checkpointErr == nil {
			checkpointErr = err
		}
		if progress.Final || progress.BestKeyID != "" {
			fmt.Fprintf(
				cmd.OutOrStdout(),
				"progress: attempts=%d rate=%.0f/s best_run=%d key_id=%s elapsed=%s\n",
				progress.Attempts, progress.Rate, progress.BestRun, progress.BestKeyID,
				progress.Elapsed.Round(time.Second),
			)
		}
	})
	if checkpointErr != nil && (result == nil || result.Candidate == nil) {
		return checkpointErr
	}
	if checkpointErr != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: progress checkpoint failed before finalization: %v\n", checkpointErr)
	}
	if result == nil {
		if searchErr != nil {
			return searchErr
		}
		return fmt.Errorf("vanity search returned no result")
	}

	if result.Candidate == nil {
		fmt.Fprintf(
			cmd.OutOrStdout(),
			"no improvement over checkpoint best_run=%d after %d new attempts\n",
			checkpoint.BestRun, result.RunAttempts,
		)
		if searchErr != nil && searchErr != context.Canceled {
			return searchErr
		}
		return nil
	}

	encryptor, err := service.NewPGPEncryptor(appInstance.Config.KeyGeneration.EncryptorPublicKey)
	if err != nil {
		return fmt.Errorf("initialize vanity key encryptor: %w", err)
	}
	artifacts, err := vanity.FinalizeAndWrite(
		vanityOutputDir,
		vanity.Identity{
			Name:    appInstance.Config.KeyGeneration.Name,
			Comment: appInstance.Config.KeyGeneration.Comment,
			Email:   appInstance.Config.KeyGeneration.Email,
		},
		*result.Candidate,
		primaryCreatedAt,
		result,
		scope,
		targetDigits,
		encryptor,
	)
	if err != nil {
		return fmt.Errorf("finalize vanity signing key: %w", err)
	}

	checkpoint.Attempts = result.Attempts
	checkpoint.BestRun = artifacts.Metadata.RunLength
	checkpoint.BestKeyID = artifacts.Metadata.SigningKeyID
	checkpoint.BestSigningFingerprint = artifacts.Metadata.SigningSubkeyFingerprint
	checkpoint.LatestPublicKeyPath = artifacts.PublicKeyPath
	checkpoint.LatestEncryptedPrivatePath = artifacts.EncryptedPrivatePath
	checkpoint.LatestMetadataPath = artifacts.MetadataPath
	if err := vanity.SaveCheckpoint(checkpointPath, *checkpoint); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "vanity signing subkey ready: key_id=%s run=%d digit=%s\n", artifacts.Metadata.SigningKeyID, artifacts.Metadata.RunLength, artifacts.Metadata.RepeatedDigit)
	fmt.Fprintf(cmd.OutOrStdout(), "public key: %s\n", artifacts.PublicKeyPath)
	fmt.Fprintf(cmd.OutOrStdout(), "encrypted private key: %s\n", artifacts.EncryptedPrivatePath)
	fmt.Fprintf(cmd.OutOrStdout(), "metadata: %s\n", artifacts.MetadataPath)
	fmt.Fprintln(cmd.OutOrStdout(), "decrypt the private artifact with GnuPG before importing; never commit it to source control")

	if searchErr != nil && searchErr != context.Canceled {
		return searchErr
	}
	return nil
}

func checkpointHasSearchState(checkpoint *vanity.Checkpoint) bool {
	return checkpoint.Attempts > 0 || checkpoint.BestRun > 0 || checkpoint.BestKeyID != ""
}

func init() {
	RootCmd.AddCommand(VanityCmd)
	VanityCmd.Flags().IntVar(&vanityMinRun, "min-run", 8, "stop after finding at least this many repeated hexadecimal digits (1-16)")
	VanityCmd.Flags().IntVarP(&vanityWorkers, "workers", "j", 0, "search workers (0 uses config or logical CPU count)")
	VanityCmd.Flags().StringVar(&vanityScope, "scope", string(vanity.ScopeSuffix), "match scope: suffix or any")
	VanityCmd.Flags().StringVar(&vanityDigits, "digits", vanity.AllDigits.String(), "hexadecimal digits allowed to form the repeated run (for example: 180 or 1,8,0)")
	VanityCmd.Flags().Uint64Var(&vanityMaxAttempts, "max-attempts", 0, "maximum attempts in this run (0 searches until target or cancellation)")
	VanityCmd.Flags().DurationVar(&vanityTimestampWindow, "timestamp-window", 30*24*time.Hour, "historical timestamp range scanned for each Ed25519 key")
	VanityCmd.Flags().StringVarP(&vanityOutputDir, "output-dir", "o", "./vanity_keys", "directory for generated key artifacts")
	VanityCmd.Flags().StringVar(&vanityCheckpointPath, "checkpoint", "", "checkpoint file (default: <output-dir>/vanity-checkpoint.json)")
	VanityCmd.Flags().BoolVar(&vanityResume, "resume", true, "resume attempt and best-run counters from the checkpoint")
	VanityCmd.Flags().DurationVar(&vanityProgressInterval, "progress-interval", 5*time.Second, "progress and checkpoint interval")
}
