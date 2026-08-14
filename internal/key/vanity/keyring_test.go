package vanity

import (
	"bytes"
	"crypto/sha1"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSigningKeyringRoundTrips(t *testing.T) {
	candidate := testCandidate(t)
	primaryCreatedAt := time.Unix(int64(candidate.Timestamp)-3600, 0)
	entity, err := BuildSigningKeyring(Identity{
		Name:  "Vanity Test",
		Email: "vanity@example.com",
	}, candidate, primaryCreatedAt)
	require.NoError(t, err)
	require.NoError(t, ValidateSigningKeyring(entity, candidate.KeyID))

	var publicArmor bytes.Buffer
	armorWriter, err := armor.Encode(&publicArmor, openpgp.PublicKeyType, nil)
	require.NoError(t, err)
	require.NoError(t, entity.Serialize(armorWriter))
	require.NoError(t, armorWriter.Close())

	parsed, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(publicArmor.Bytes()))
	require.NoError(t, err)
	require.Len(t, parsed, 1)
	require.Len(t, parsed[0].Subkeys, 1)
	assert.Equal(t, candidate.KeyID, parsed[0].Subkeys[0].PublicKey.KeyId)
	assert.True(t, parsed[0].Subkeys[0].Sig.FlagSign)
	require.NoError(t, ValidateSigningKeyring(parsed[0], candidate.KeyID))
}

func TestGnuPGImportsAndSignsWithGeneratedKeyring(t *testing.T) {
	gpgPath := findGPG()
	if gpgPath == "" {
		t.Skip("GnuPG is not installed")
	}

	candidate := testCandidate(t)
	entity, err := BuildSigningKeyring(Identity{
		Name:  "Vanity GnuPG Test",
		Email: "vanity-gpg@example.com",
	}, candidate, time.Unix(int64(candidate.Timestamp)-3600, 0))
	require.NoError(t, err)

	var publicArmor bytes.Buffer
	armorWriter, err := armor.Encode(&publicArmor, openpgp.PublicKeyType, nil)
	require.NoError(t, err)
	require.NoError(t, entity.Serialize(armorWriter))
	require.NoError(t, armorWriter.Close())
	var privateArmor bytes.Buffer
	privateArmorWriter, err := armor.Encode(&privateArmor, openpgp.PrivateKeyType, nil)
	require.NoError(t, err)
	require.NoError(t, entity.SerializePrivate(privateArmorWriter, nil))
	require.NoError(t, privateArmorWriter.Close())

	gnupgHome := filepath.Join(t.TempDir(), "gnupg")
	require.NoError(t, os.MkdirAll(gnupgHome, 0o700))
	publicPath := filepath.Join(t.TempDir(), "vanity-public.asc")
	require.NoError(t, os.WriteFile(publicPath, publicArmor.Bytes(), 0o600))
	privatePath := filepath.Join(t.TempDir(), "vanity-private.asc")
	require.NoError(t, os.WriteFile(privatePath, privateArmor.Bytes(), 0o600))

	command := exec.Command(gpgPath, "--batch", "--homedir", gnupgHome, "--import-options", "show-only", "--import", publicPath)
	output, err := command.CombinedOutput()
	require.NoError(t, err, string(output))
	assert.Contains(t, string(output), candidate.KeyIDHex())

	command = exec.Command(gpgPath, "--batch", "--homedir", gnupgHome, "--import", privatePath)
	output, err = command.CombinedOutput()
	require.NoError(t, err, string(output))

	messagePath := filepath.Join(t.TempDir(), "message.txt")
	signaturePath := filepath.Join(t.TempDir(), "message.txt.asc")
	require.NoError(t, os.WriteFile(messagePath, []byte("gpgenie GnuPG interoperability test\n"), 0o600))
	command = exec.Command(
		gpgPath,
		"--batch", "--yes", "--homedir", gnupgHome,
		"--local-user", candidate.FingerprintHex()+"!",
		"--armor", "--detach-sign", "--output", signaturePath,
		messagePath,
	)
	output, err = command.CombinedOutput()
	require.NoError(t, err, string(output))

	command = exec.Command(gpgPath, "--batch", "--homedir", gnupgHome, "--status-fd", "1", "--verify", signaturePath, messagePath)
	output, err = command.CombinedOutput()
	require.NoError(t, err, string(output))
	assert.Contains(t, string(output), "VALIDSIG "+candidate.FingerprintHex())
}

func testCandidate(t *testing.T) Candidate {
	t.Helper()
	privateKey, template, err := generateCandidateKey()
	require.NoError(t, err)
	timestamp := uint32(time.Now().Add(-time.Hour).Unix())
	fingerprint, keyID, err := fingerprintAt(template, timestamp)
	require.NoError(t, err)
	require.Len(t, fingerprint, sha1.Size)
	return Candidate{
		Fingerprint: fingerprint,
		KeyID:       keyID,
		Timestamp:   timestamp,
		Match:       EvaluateKeyID(keyID, ScopeSuffix),
		privateKey:  privateKey,
	}
}

func findGPG() string {
	if path, err := exec.LookPath("gpg"); err == nil {
		return path
	}
	if runtime.GOOS == "windows" {
		for _, candidate := range []string{
			`C:\Program Files\GnuPG\bin\gpg.exe`,
			`C:\Program Files\Git\usr\bin\gpg.exe`,
		} {
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}
	return ""
}
