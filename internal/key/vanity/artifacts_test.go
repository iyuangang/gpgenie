package vanity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testArtifactEncryptor struct{}

func (testArtifactEncryptor) Encrypt(plaintext string) (string, error) {
	return "encrypted:" + plaintext, nil
}

func TestFinalizeAndWriteProducesEncryptedArtifacts(t *testing.T) {
	candidate := testCandidate(t)
	result := &SearchResult{
		Candidate:   &candidate,
		Attempts:    1234,
		RunAttempts: 1234,
		BestRun:     candidate.Match.RunLength,
		Elapsed:     time.Second,
		Rate:        1234,
	}
	outputDir := filepath.Join(t.TempDir(), "vanity-output")
	artifacts, err := FinalizeAndWrite(
		outputDir,
		Identity{Name: "Artifact Test", Email: "artifact@example.com"},
		candidate,
		time.Unix(int64(candidate.Timestamp)-3600, 0),
		result,
		ScopeSuffix,
		"018",
		testArtifactEncryptor{},
	)
	require.NoError(t, err)

	publicData, err := os.ReadFile(artifacts.PublicKeyPath)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(publicData), "-----BEGIN PGP PUBLIC KEY BLOCK-----"))
	privateData, err := os.ReadFile(artifacts.EncryptedPrivatePath)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(privateData), "encrypted:-----BEGIN PGP PRIVATE KEY BLOCK-----"))
	assert.FileExists(t, artifacts.MetadataPath)
	assert.Equal(t, candidate.KeyIDHex(), artifacts.Metadata.SigningKeyID)
	assert.Equal(t, "018", artifacts.Metadata.TargetDigits)
}

func TestCheckpointRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "checkpoint.json")
	want := Checkpoint{
		Attempts:               987654,
		BestRun:                9,
		Scope:                  ScopeSuffix,
		TargetDigits:           "018",
		BestKeyID:              "ABCDEF1234999999",
		LatestPublicKeyPath:    "public.asc",
		LatestMetadataPath:     "result.json",
		BestSigningFingerprint: "0123456789ABCDEF01234567ABCDEF1234999999",
	}
	require.NoError(t, SaveCheckpoint(path, want))

	got, err := LoadCheckpoint(path)
	require.NoError(t, err)
	assert.Equal(t, want.Attempts, got.Attempts)
	assert.Equal(t, want.BestRun, got.BestRun)
	assert.Equal(t, want.BestKeyID, got.BestKeyID)
	assert.Equal(t, want.Scope, got.Scope)
	assert.Equal(t, want.TargetDigits, got.TargetDigits)
	assert.NotEmpty(t, got.UpdatedAt)
}

func TestLoadMissingCheckpointReturnsEmptyState(t *testing.T) {
	checkpoint, err := LoadCheckpoint(filepath.Join(t.TempDir(), "missing.json"))
	require.NoError(t, err)
	assert.Equal(t, &Checkpoint{}, checkpoint)
}
