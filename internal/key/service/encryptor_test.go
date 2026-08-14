package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPGPEncryptorRejectsOpenSSHKeyWithActionableError(t *testing.T) {
	_, err := newPGPEncryptor([]byte("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCexample user@host"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "OpenSSH public key")
	assert.Contains(t, err.Error(), armoredPublicKeyHeader)
	assert.Contains(t, err.Error(), ".ssh/*.pub")
}

func TestNewPGPEncryptorRejectsEmptyKeyWithActionableError(t *testing.T) {
	_, err := newPGPEncryptor([]byte(" \r\n\t"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "public key file is empty")
	assert.Contains(t, err.Error(), armoredPublicKeyHeader)
}

func TestNewPGPEncryptorRejectsUnsupportedKeyFormat(t *testing.T) {
	_, err := newPGPEncryptor([]byte("not a public key"))

	require.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "unsupported public key format"))
	assert.Contains(t, err.Error(), armoredPublicKeyHeader)
}
