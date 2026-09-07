//go:build windows && (amd64 || arm64)

package vanity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRetentionSafeGPUWorkItems(t *testing.T) {
	assert.Equal(t, uint64(1024), retentionSafeGPUWorkItems(defaultGPUWorkItems, 1, ScopeSuffix))
	assert.Equal(t, uint64(16384), retentionSafeGPUWorkItems(defaultGPUWorkItems, 2, ScopeSuffix))
	assert.Equal(t, uint64(1024), retentionSafeGPUWorkItems(defaultGPUWorkItems, 2, ScopeAny))
	assert.Equal(t, defaultGPUWorkItems, retentionSafeGPUWorkItems(defaultGPUWorkItems, 12, ScopeSuffix))
}
