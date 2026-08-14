package vanity

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluateKeyIDSuffix(t *testing.T) {
	tests := []struct {
		name  string
		keyID uint64
		want  Match
	}{
		{name: "seven trailing nines", keyID: 0x5A51FDB219999999, want: Match{RunLength: 7, Start: 9, Digit: 9}},
		{name: "all equal", keyID: 0xAAAAAAAAAAAAAAAA, want: Match{RunLength: 16, Start: 0, Digit: 10}},
		{name: "single trailing digit", keyID: 0x0123456789ABCDEF, want: Match{RunLength: 1, Start: 15, Digit: 15}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, EvaluateKeyID(tt.keyID, ScopeSuffix))
		})
	}
}

func TestEvaluateKeyIDAny(t *testing.T) {
	match := EvaluateKeyID(0x11111ABCDEF99999, ScopeAny)
	require.Equal(t, 5, match.RunLength)
	// Equal runs prefer the one furthest to the right so suffix-like results
	// win ties in the full 16-digit GitHub display.
	assert.Equal(t, 11, match.Start)
	assert.Equal(t, byte(9), match.Digit)
}

func TestParseDigits(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "180", want: "018"},
		{input: "1, 8, 0", want: "018"},
		{input: "f,A,a", want: "AF"},
		{input: "0123456789abcdef", want: "0123456789ABCDEF"},
	}

	for _, tt := range tests {
		got, err := ParseDigits(tt.input)
		require.NoError(t, err)
		assert.Equal(t, tt.want, got.String())
	}

	_, err := ParseDigits("")
	require.Error(t, err)
	_, err = ParseDigits(", ,")
	require.Error(t, err)
	_, err = ParseDigits("18G")
	require.Error(t, err)
}

func TestEvaluateKeyIDForSelectedDigits(t *testing.T) {
	allowed, err := ParseDigits("1,8,0")
	require.NoError(t, err)

	assert.Equal(t,
		Match{RunLength: 0, Start: 15, Digit: 14},
		EvaluateKeyIDForDigits(0xE47E8AFEEEEEEEEE, ScopeSuffix, allowed),
	)
	assert.Equal(t,
		Match{RunLength: 10, Start: 6, Digit: 1},
		EvaluateKeyIDForDigits(0xABCDEF1111111111, ScopeSuffix, allowed),
	)
	assert.Equal(t,
		Match{RunLength: 5, Start: 0, Digit: 1},
		EvaluateKeyIDForDigits(0x11111ABCDEF99999, ScopeAny, allowed),
	)
	assert.Equal(t, 0, EvaluateKeyIDForDigits(0xABCDEFABCDEFABCD, ScopeAny, allowed).RunLength)
}

func TestScopeValidation(t *testing.T) {
	require.NoError(t, ScopeSuffix.Validate())
	require.NoError(t, ScopeAny.Validate())
	require.Error(t, Scope("invalid").Validate())
}
