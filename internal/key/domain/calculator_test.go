package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalculateScores(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Scores
	}{
		{
			name:  "repeated letters",
			input: "AAAA123",
			expected: Scores{
				RepeatLetterScore:     128, // 4^1.5 * 16
				IncreasingLetterScore: 0,
				DecreasingLetterScore: 0,
				MagicLetterScore:      0,
				UniqueLettersCount:    4,
			},
		},
		{
			name:  "increasing sequence",
			input: "0123456",
			expected: Scores{
				RepeatLetterScore:     0,
				IncreasingLetterScore: 224, // 6^1.5 * 16
				DecreasingLetterScore: 0,
				MagicLetterScore:      0,
				UniqueLettersCount:    7,
			},
		},
		{
			name:  "decreasing sequence",
			input: "FEDCBA",
			expected: Scores{
				RepeatLetterScore:     0,
				IncreasingLetterScore: 0,
				DecreasingLetterScore: 176, // 5^1.5 * 16
				MagicLetterScore:      0,
				UniqueLettersCount:    6,
			},
		},
		{
			name:  "magic sequence",
			input: "49ABCD",
			expected: Scores{
				RepeatLetterScore:     0,
				IncreasingLetterScore: 128,
				DecreasingLetterScore: 0,
				MagicLetterScore:      -100,
				UniqueLettersCount:    6,
			},
		},
		{
			name:  "mixed case",
			input: "aAaA123",
			expected: Scores{
				RepeatLetterScore:     128, // 4^1.5 * 16
				IncreasingLetterScore: 0,
				DecreasingLetterScore: 0,
				MagicLetterScore:      0,
				UniqueLettersCount:    4,
			},
		},
		{
			name:  "circular increasing",
			input: "FEDF012",
			expected: Scores{
				RepeatLetterScore:     0,
				IncreasingLetterScore: 80,
				DecreasingLetterScore: 0, // 4^1.5 * 16
				MagicLetterScore:      0,
				UniqueLettersCount:    6,
			},
		},
		{
			name:  "invalid characters",
			input: "ABC!@#123",
			expected: Scores{
				RepeatLetterScore:     0,
				IncreasingLetterScore: 0,
				DecreasingLetterScore: 0,
				MagicLetterScore:      0,
				UniqueLettersCount:    6,
			},
		},
		{
			name:  "long repeated sequence",
			input: "AAAAAAAAAAAAAAAA",
			expected: Scores{
				RepeatLetterScore:     1024, // 16^1.5 * 16 (capped at 16)
				IncreasingLetterScore: 0,
				DecreasingLetterScore: 0,
				MagicLetterScore:      0,
				UniqueLettersCount:    1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scores, err := CalculateScores(tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, scores)
		})
	}
}

func TestCharToValue(t *testing.T) {
	tests := []struct {
		input    byte
		expected int
		valid    bool
	}{
		{'0', 0, true},
		{'9', 9, true},
		{'A', 10, true},
		{'F', 15, true},
		{'a', 10, true},
		{'f', 15, true},
		{'G', 0, false},
		{'g', 0, false},
		{'!', 0, false},
		{byte(128), 0, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			val, ok := charToValue(tt.input)
			assert.Equal(t, tt.expected, val)
			assert.Equal(t, tt.valid, ok)
		})
	}
}

func TestIsIncreasing(t *testing.T) {
	tests := []struct {
		a, b     byte
		expected bool
	}{
		{'0', '1', true},
		{'9', 'A', true},
		{'F', '0', true},  // circular
		{'f', '0', true},  // lowercase
		{'A', 'C', false}, // non-sequential
		{'!', '1', false}, // invalid char
		{'1', '!', false}, // invalid char
	}

	for _, tt := range tests {
		t.Run(string(tt.a)+string(tt.b), func(t *testing.T) {
			result := isIncreasing(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsDecreasing(t *testing.T) {
	tests := []struct {
		a, b     byte
		expected bool
	}{
		{'1', '0', true},
		{'A', '9', true},
		{'0', 'F', true},  // circular
		{'0', 'f', true},  // lowercase
		{'C', 'A', false}, // non-sequential
		{'!', '1', false}, // invalid char
		{'1', '!', false}, // invalid char
	}

	for _, tt := range tests {
		t.Run(string(tt.a)+string(tt.b), func(t *testing.T) {
			result := isDecreasing(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// 基准测试
func BenchmarkCalculateScores(b *testing.B) {
	inputs := []string{
		"0123456789ABCDEF", // 所有可能的字符
		"AAAAAAAAAAAAAAAA", // 重复字符
		"0123456789ABCDEF", // 递增序列
		"FEDCBA9876543210", // 递减序列
		"49ABCDEF01234567", // 包含魔法序列
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, input := range inputs {
			_, _ = CalculateScores(input)
		}
	}
}

func BenchmarkCharToValue(b *testing.B) {
	chars := []byte{'0', '9', 'A', 'F', 'a', 'f', 'G', '!'}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, c := range chars {
			_, _ = charToValue(c)
		}
	}
}

func BenchmarkIsIncreasing(b *testing.B) {
	pairs := [][2]byte{
		{'0', '1'},
		{'9', 'A'},
		{'F', '0'},
		{'A', 'C'},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, pair := range pairs {
			isIncreasing(pair[0], pair[1])
		}
	}
}

func BenchmarkIsDecreasing(b *testing.B) {
	pairs := [][2]byte{
		{'1', '0'},
		{'A', '9'},
		{'0', 'F'},
		{'C', 'A'},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, pair := range pairs {
			isDecreasing(pair[0], pair[1])
		}
	}
}
