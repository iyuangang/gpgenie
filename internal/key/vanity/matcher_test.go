package vanity

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatcherAndFilterAgainstHexReference(t *testing.T) {
	rng := rand.New(rand.NewPCG(4, 5))
	for sample := 0; sample < 10000; sample++ {
		keyID := rng.Uint64()
		// Force long runs at every possible position as well as random IDs.
		if sample%2 == 0 {
			run := 1 + rng.IntN(16)
			shift := uint(rng.IntN(17-run) * 4)
			mask := ^uint64(0) >> uint((16-run)*4)
			keyID = keyID & ^(mask<<shift) | (rng.Uint64()&15)*0x1111111111111111&mask<<shift
		}
		for _, scope := range []Scope{ScopeSuffix, ScopeAny} {
			for _, digits := range []DigitSet{AllDigits, 0, 0x103, 0x8000, 0x1} {
				want := referenceMatch(keyID, scope, digits)
				require.Equal(t, want, EvaluateKeyIDForDigits(keyID, scope, digits))
			}
			want := referenceMatch(keyID, scope, AllDigits)
			for minRun := 1; minRun <= 16; minRun++ {
				require.Equal(t, want.RunLength >= minRun, newKeyIDFilter(scope, minRun).allows(keyID), "key=%016x scope=%s minRun=%d", keyID, scope, minRun)
			}
		}
	}
}

func referenceMatch(keyID uint64, scope Scope, digits DigitSet) Match {
	if digits == 0 {
		digits = AllDigits
	}
	hex := Candidate{KeyID: keyID}.KeyIDHex()
	if scope == ScopeSuffix {
		digit := byte(strings.IndexByte("0123456789ABCDEF", hex[15]))
		start := 15
		if !digits.Allows(digit) {
			return Match{Start: start, Digit: digit}
		}
		for start > 0 && hex[start-1] == hex[15] {
			start--
		}
		return Match{Start: start, RunLength: 16 - start, Digit: digit}
	}
	best := Match{Digit: byte(strings.IndexByte("0123456789ABCDEF", hex[0]))}
	for start := 0; start < 16; {
		end := start + 1
		for end < 16 && hex[end] == hex[start] {
			end++
		}
		digit := byte(strings.IndexByte("0123456789ABCDEF", hex[start]))
		if digits.Allows(digit) && end-start >= best.RunLength {
			best = Match{Start: start, RunLength: end - start, Digit: digit}
		}
		start = end
	}
	return best
}

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
