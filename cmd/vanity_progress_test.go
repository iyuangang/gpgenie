package cmd

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/iyuangang/gpgenie/internal/key/vanity"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVanityProgressDisplayRedrawsOneTerminalLine(t *testing.T) {
	var output bytes.Buffer
	display := &vanityProgressDisplay{output: &output, inline: true}
	display.Update("progress: long", false)
	display.Update("done", true)

	assert.Equal(t, "\rprogress: long\rdone          \n", output.String())
}

func TestVanityProgressDisplayUsesLinesWhenRedirected(t *testing.T) {
	var output bytes.Buffer
	display := &vanityProgressDisplay{output: &output}
	display.Update("first", false)
	display.Update("second", true)

	assert.Equal(t, "first\nsecond\n", output.String())
}

func TestFormatVanityProgress(t *testing.T) {
	line := formatVanityProgress(vanity.Progress{
		Attempts:    2_336_633_662_306,
		RunAttempts: 335_955_000,
		BestRun:     10,
		Elapsed:     5 * time.Second,
		Rate:        67_191_000,
	}, 15, "A5E91B8888888888", mathPow16(14))

	assert.True(t, strings.Contains(line, "total=2.337T"))
	assert.True(t, strings.Contains(line, "+335.955M"))
	assert.True(t, strings.Contains(line, "67.191M/s"))
	assert.True(t, strings.Contains(line, "best=10/15"))
	assert.True(t, strings.Contains(line, "key=A5E91B8888888888"))
	assert.True(t, strings.Contains(line, "eta~34.0y"))
}

func TestExpectedVanityAttempts(t *testing.T) {
	assert.Equal(t, mathPow16(14), expectedVanityAttempts(15, vanity.ScopeSuffix, vanity.AllDigits))
	assert.Equal(t, mathPow16(15)/3, expectedVanityAttempts(15, vanity.ScopeSuffix, mustDigits(t, "018")))
	assert.Zero(t, expectedVanityAttempts(15, vanity.ScopeAny, vanity.AllDigits))
}

func mathPow16(exponent int) float64 {
	result := float64(1)
	for range exponent {
		result *= 16
	}
	return result
}

func mustDigits(t *testing.T, value string) vanity.DigitSet {
	t.Helper()
	digits, err := vanity.ParseDigits(value)
	if err != nil {
		t.Fatal(err)
	}
	return digits
}

func TestParseOpenCLDeviceSelection(t *testing.T) {
	for _, value := range []string{"", "all", "ALL"} {
		devices, err := parseOpenCLDeviceSelection(value)
		require.NoError(t, err)
		assert.Empty(t, devices)
	}
	devices, err := parseOpenCLDeviceSelection("0, 1;1")
	require.NoError(t, err)
	assert.Equal(t, []int{0, 1}, devices)
	for _, value := range []string{"-1", "gpu0", ",,,"} {
		_, err := parseOpenCLDeviceSelection(value)
		assert.Error(t, err)
	}
}
