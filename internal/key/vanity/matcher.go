package vanity

import (
	"fmt"
	"strings"
	"unicode"
)

// Scope controls where a repeated hexadecimal run is measured.
type Scope string

const (
	ScopeSuffix Scope = "suffix"
	ScopeAny    Scope = "any"

	// AllDigits allows every hexadecimal digit. DigitSet is a bit mask so the
	// search hot path can filter candidates without string conversions.
	AllDigits DigitSet = 0xffff
)

// DigitSet identifies the hexadecimal digits whose repeated runs are useful.
// Bit n corresponds to hexadecimal digit n.
type DigitSet uint16

// Match describes the longest repeated hexadecimal run in a 64-bit key ID.
// Start is zero-based from the most-significant hexadecimal digit.
type Match struct {
	RunLength int  `json:"run_length"`
	Start     int  `json:"start"`
	Digit     byte `json:"digit"`
}

func (s Scope) Validate() error {
	switch s {
	case ScopeSuffix, ScopeAny:
		return nil
	default:
		return fmt.Errorf("scope must be %q or %q", ScopeSuffix, ScopeAny)
	}
}

// ParseDigits accepts compact hexadecimal digits ("180") or comma/space
// separated digits ("1, 8, 0"). Duplicate digits are ignored.
func ParseDigits(value string) (DigitSet, error) {
	var result DigitSet
	for _, char := range strings.TrimSpace(value) {
		var digit byte
		switch {
		case char >= '0' && char <= '9':
			digit = byte(char - '0')
		case char >= 'a' && char <= 'f':
			digit = byte(char-'a') + 10
		case char >= 'A' && char <= 'F':
			digit = byte(char-'A') + 10
		case char == ',' || unicode.IsSpace(char):
			continue
		default:
			return 0, fmt.Errorf("digits must contain only hexadecimal digits, commas, or spaces; got %q", char)
		}
		result |= 1 << digit
	}
	if result == 0 {
		return 0, fmt.Errorf("digits must contain at least one hexadecimal digit")
	}
	return result, nil
}

func (d DigitSet) Allows(digit byte) bool {
	return digit < 16 && d&(1<<digit) != 0
}

// String returns a stable canonical representation suitable for checkpoints.
func (d DigitSet) String() string {
	var result strings.Builder
	for digit := byte(0); digit < 16; digit++ {
		if d.Allows(digit) {
			result.WriteString(formatHexDigit(digit))
		}
	}
	return result.String()
}

// EvaluateKeyID examines the key ID without allocating or converting it to a
// hexadecimal string. GitHub displays the 16 hexadecimal digits represented
// by this value for a version 4 OpenPGP signing key.
func EvaluateKeyID(keyID uint64, scope Scope) Match {
	return EvaluateKeyIDForDigits(keyID, scope, AllDigits)
}

// EvaluateKeyIDForDigits is EvaluateKeyID restricted to runs made from the
// allowed digits. A zero DigitSet means all digits for backwards-compatible
// SearchConfig zero values.
func EvaluateKeyIDForDigits(keyID uint64, scope Scope, allowed DigitSet) Match {
	if allowed == 0 {
		allowed = AllDigits
	}
	if scope == ScopeSuffix {
		digit := byte(keyID & 0x0f)
		if !allowed.Allows(digit) {
			return Match{Start: 15, Digit: digit}
		}
		run := 1
		for i := 1; i < 16; i++ {
			if byte((keyID>>uint(i*4))&0x0f) != digit {
				break
			}
			run++
		}
		return Match{RunLength: run, Start: 16 - run, Digit: digit}
	}

	currentDigit := byte((keyID >> 60) & 0x0f)
	best := Match{Start: 0, Digit: currentDigit}
	if allowed.Allows(currentDigit) {
		best.RunLength = 1
	}
	currentStart := 0
	currentRun := 1
	for i := 1; i < 16; i++ {
		digit := byte((keyID >> uint((15-i)*4)) & 0x0f)
		if digit == currentDigit {
			currentRun++
		} else {
			currentStart = i
			currentDigit = digit
			currentRun = 1
		}
		if allowed.Allows(currentDigit) && (currentRun > best.RunLength ||
			(currentRun == best.RunLength && currentStart > best.Start)) {
			best = Match{RunLength: currentRun, Start: currentStart, Digit: currentDigit}
		}
	}
	return best
}

func formatHexDigit(digit byte) string {
	const digits = "0123456789ABCDEF"
	if digit > 15 {
		return "?"
	}
	return string(digits[digit])
}
