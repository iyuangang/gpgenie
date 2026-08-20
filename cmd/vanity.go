package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/iyuangang/gpgenie/internal/app"
	"github.com/iyuangang/gpgenie/internal/key/service"
	"github.com/iyuangang/gpgenie/internal/key/vanity"
	"github.com/iyuangang/gpgenie/internal/repository"

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
	vanitySaveToDatabase   bool
	vanityProgressInterval time.Duration
	vanityBackend          string
	vanityOpenCLDevices    string
	vanityGPUKeyBatch      int
	vanityGPUWorkItems     uint64
	vanityListOpenCL       bool
)

var VanityCmd = &cobra.Command{
	Use:   "vanity",
	Short: "mine a real OpenPGP vanity signing subkey",
	Long: `Mine an Ed25519 OpenPGP signing subkey whose 16-digit long key ID
contains a repeated hexadecimal run. The normal primary key is generated only
after a result is found and the private keyring is encrypted to the configured
encryptor_public_key. The OpenCL backend uses every selected GPU concurrently
and verifies every GPU winner again on the CPU.`,
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
	backendName := vanityBackend
	if !cmd.Flags().Changed("backend") && appInstance.Config.Vanity.Backend != "" {
		backendName = appInstance.Config.Vanity.Backend
	}
	backend := vanity.Backend(strings.ToLower(strings.TrimSpace(backendName)))
	deviceSelection := vanityOpenCLDevices
	if !cmd.Flags().Changed("gpu-devices") && appInstance.Config.Vanity.OpenCLDevices != "" {
		deviceSelection = appInstance.Config.Vanity.OpenCLDevices
	}
	selectedDevices, err := parseOpenCLDeviceSelection(deviceSelection)
	if err != nil {
		return fmt.Errorf("invalid --gpu-devices: %w", err)
	}
	gpuKeyBatch := vanityGPUKeyBatch
	if !cmd.Flags().Changed("gpu-key-batch") && appInstance.Config.Vanity.GPUKeyBatch != 0 {
		gpuKeyBatch = appInstance.Config.Vanity.GPUKeyBatch
	}
	gpuWorkItems := vanityGPUWorkItems
	if !cmd.Flags().Changed("gpu-work-items") && appInstance.Config.Vanity.GPUWorkItems != 0 {
		gpuWorkItems = appInstance.Config.Vanity.GPUWorkItems
	}
	if vanityListOpenCL {
		devices, err := vanity.ListOpenCLDevices()
		if err != nil {
			return err
		}
		if len(devices) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "no OpenCL GPU devices found")
			return nil
		}
		for _, device := range devices {
			fmt.Fprintf(cmd.OutOrStdout(), "[%d] %s | platform=%s vendor=%s compute_units=%d memory=%.1fGiB driver=%s opencl=%s\n",
				device.Index, device.Name, device.Platform, device.Vendor, device.ComputeUnits,
				float64(device.GlobalMemoryBytes)/(1024*1024*1024), device.DriverVersion, device.OpenCLVersion)
		}
		return nil
	}
	effectiveBackend, openCLDevices, err := vanity.ResolveBackend(backend, selectedDevices)
	if err != nil {
		return err
	}
	if vanityTimestampWindow < time.Second {
		return fmt.Errorf("timestamp-window must be at least one second")
	}
	minRun := vanityMinRun
	if !cmd.Flags().Changed("min-run") && appInstance.Config.Vanity.MinRun != 0 {
		minRun = appInstance.Config.Vanity.MinRun
	}
	if minRun < 1 || minRun > 16 {
		return fmt.Errorf("min-run must be between 1 and 16")
	}
	saveToDatabase := vanitySaveToDatabase
	if !cmd.Flags().Changed("save-db") {
		saveToDatabase = appInstance.Config.Vanity.SaveToDatabase
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
	if checkpoint.BestRun >= minRun {
		if saveToDatabase && !checkpoint.SavedToDatabase {
			artifacts, err := vanity.LoadArtifacts(
				checkpoint.LatestPublicKeyPath,
				checkpoint.LatestEncryptedPrivatePath,
				checkpoint.LatestMetadataPath,
			)
			if err != nil {
				return fmt.Errorf("reload checkpoint artifacts for database save: %w", err)
			}
			if err := saveVanityToDatabase(appInstance.Repository, artifacts); err != nil {
				return err
			}
			checkpoint.SavedToDatabase = true
			if err := vanity.SaveCheckpoint(checkpointPath, *checkpoint); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "checkpoint vanity key saved to database: fingerprint=%s\n", artifacts.Metadata.SigningSubkeyFingerprint)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "checkpoint already contains run=%d key_id=%s\n", checkpoint.BestRun, checkpoint.BestKeyID)
		return nil
	}

	cpuWorkers := 0
	if effectiveBackend == vanity.BackendCPU || effectiveBackend == vanity.BackendHybrid {
		cpuWorkers = workers
	}
	fmt.Fprintf(
		cmd.OutOrStdout(),
		"vanity search started: backend=%s cpu_workers=%d opencl_devices=%d scope=%s digits=%s target_run=%d save_db=%t timestamp_window=%s previous_attempts=%d\n",
		effectiveBackend, cpuWorkers,
		len(openCLDevices), scope, targetDigits, minRun, saveToDatabase, vanityTimestampWindow, checkpoint.Attempts,
	)
	for _, device := range openCLDevices {
		fmt.Fprintf(cmd.OutOrStdout(), "OpenCL GPU [%d]: %s (%s), compute_units=%d memory=%.1fGiB driver=%s\n",
			device.Index, device.Name, device.Platform, device.ComputeUnits,
			float64(device.GlobalMemoryBytes)/(1024*1024*1024), device.DriverVersion)
	}
	fmt.Fprintf(
		cmd.OutOrStdout(),
		"key timestamps will span %s through %s; each additional repeated digit costs about 16x more work\n",
		start.Format(time.RFC3339), now.Format(time.RFC3339),
	)

	searchConfig := vanity.SearchConfig{
		Backend:          effectiveBackend,
		Workers:          workers,
		OpenCLDevices:    selectedDevices,
		GPUKeyBatch:      gpuKeyBatch,
		GPUWorkItems:     gpuWorkItems,
		MinRun:           minRun,
		Scope:            scope,
		AllowedDigits:    allowedDigits,
		TimestampStart:   uint32(start.Unix()),
		TimestampEnd:     uint32(now.Unix()),
		MaxAttempts:      vanityMaxAttempts,
		InitialAttempts:  checkpoint.Attempts,
		InitialBestRun:   checkpoint.BestRun,
		ProgressInterval: vanityProgressInterval,
	}
	progressDisplay := newVanityProgressDisplay(cmd.OutOrStdout())
	expectedAttempts := expectedVanityAttempts(minRun, scope, allowedDigits)
	progressDisplay.Update(formatVanityProgress(vanity.Progress{
		Attempts: checkpoint.Attempts,
		BestRun:  checkpoint.BestRun,
	}, minRun, checkpoint.BestKeyID, expectedAttempts), false)
	defer progressDisplay.Close()

	var checkpointErr error
	result, searchErr := vanity.Search(cmd.Context(), searchConfig, func(progress vanity.Progress) {
		checkpoint.Attempts = progress.Attempts
		// Only attempts are durable while mining. A promoted candidate's
		// private material exists in memory until finalization succeeds, so do
		// not replace the durable best checkpoint prematurely.
		if err := vanity.SaveCheckpoint(checkpointPath, *checkpoint); err != nil && checkpointErr == nil {
			checkpointErr = err
		}
		bestKeyID := progress.BestKeyID
		if bestKeyID == "" {
			bestKeyID = checkpoint.BestKeyID
		}
		progressDisplay.Update(
			formatVanityProgress(progress, minRun, bestKeyID, expectedAttempts),
			progress.Final,
		)
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
	checkpoint.SavedToDatabase = false
	if err := vanity.SaveCheckpoint(checkpointPath, *checkpoint); err != nil {
		return err
	}
	targetReached := artifacts.Metadata.RunLength >= minRun
	if saveToDatabase && targetReached {
		if err := saveVanityToDatabase(appInstance.Repository, artifacts); err != nil {
			return err
		}
		checkpoint.SavedToDatabase = true
		if err := vanity.SaveCheckpoint(checkpointPath, *checkpoint); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "database: saved encrypted vanity key fingerprint=%s\n", artifacts.Metadata.SigningSubkeyFingerprint)
	}

	if targetReached {
		fmt.Fprintf(cmd.OutOrStdout(), "vanity signing subkey ready: key_id=%s run=%d digit=%s\n", artifacts.Metadata.SigningKeyID, artifacts.Metadata.RunLength, artifacts.Metadata.RepeatedDigit)
	} else {
		fmt.Fprintf(
			cmd.OutOrStdout(),
			"target not reached: preserved best candidate key_id=%s run=%d target=%d; database not updated\n",
			artifacts.Metadata.SigningKeyID,
			artifacts.Metadata.RunLength,
			minRun,
		)
	}
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

func saveVanityToDatabase(repo repository.KeyRepository, artifacts *vanity.Artifacts) error {
	record, err := artifacts.ToDatabaseKeyInfo()
	if err != nil {
		return fmt.Errorf("prepare vanity database record: %w", err)
	}
	if err := repo.Upsert(record); err != nil {
		return fmt.Errorf("save vanity key to database: %w", err)
	}
	return nil
}

func parseOpenCLDeviceSelection(value string) ([]int, error) {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "all") {
		return nil, nil
	}
	parts := strings.FieldsFunc(value, func(char rune) bool {
		return char == ',' || char == ';' || char == ' ' || char == '\t'
	})
	result := make([]int, 0, len(parts))
	seen := make(map[int]bool, len(parts))
	for _, part := range parts {
		index, err := strconv.Atoi(part)
		if err != nil || index < 0 {
			return nil, fmt.Errorf("device list must be all or non-negative indices such as 0,1; got %q", part)
		}
		if !seen[index] {
			seen[index] = true
			result = append(result, index)
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("device list is empty")
	}
	return result, nil
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
	VanityCmd.Flags().BoolVar(&vanitySaveToDatabase, "save-db", false, "save or update the matched key in the configured database (private key remains encrypted)")
	VanityCmd.Flags().DurationVar(&vanityProgressInterval, "progress-interval", 5*time.Second, "progress and checkpoint interval")
	VanityCmd.Flags().StringVar(&vanityBackend, "backend", string(vanity.BackendCPU), "search backend: cpu, opencl, hybrid, or auto")
	VanityCmd.Flags().StringVar(&vanityOpenCLDevices, "gpu-devices", "all", "OpenCL GPU indices to use concurrently (all or for example 0,1)")
	VanityCmd.Flags().IntVar(&vanityGPUKeyBatch, "gpu-key-batch", 0, "Ed25519 templates prepared per GPU batch (0 uses the tuned default)")
	VanityCmd.Flags().Uint64Var(&vanityGPUWorkItems, "gpu-work-items", 0, "hashes per OpenCL dispatch (0 uses the tuned default)")
	VanityCmd.Flags().BoolVar(&vanityListOpenCL, "list-opencl-devices", false, "list detected OpenCL GPUs and exit")
}
