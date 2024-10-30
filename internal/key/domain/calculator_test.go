package domain

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculateScores(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Scores
		wantErr  bool
	}{
		{
			name:  "empty string",
			input: "",
			expected: Scores{},
		},
		{
			name:  "single character",
			input: "A",
			expected: Scores{
				UniqueLettersCount: 1,
			},
		},
		{
			name:  "repeated letters",
			input: "AAAA",
			expected: Scores{
				RepeatLetterScore: repeatScoreMap[4],
				UniqueLettersCount: 1,
			},
		},
		{
			name:  "increasing sequence",
			input: "0123456",
			expected: Scores{
				IncreasingLetterScore: sequenceScoreMap[7],
				UniqueLettersCount:   7,
			},
		},
		{
			name:  "decreasing sequence",
			input: "FEDCBA",
			expected: Scores{
				DecreasingLetterScore: sequenceScoreMap[6],
				UniqueLettersCount:   6,
			},
		},
		{
			name:  "magic sequence",
			input: "49ABCD",
			expected: Scores{
				MagicLetterScore: magicScore,
				IncreasingLetterScore: sequenceScoreMap[5],
				UniqueLettersCount: 6,
			},
		},
		{
			name:  "mixed case",
			input: "aAbBcC",
			expected: Scores{
				UniqueLettersCount: 3,
			},
		},
		{
			name:  "invalid characters",
			input: "!@#$%^",
			expected: Scores{},
		},
		{
			name:  "complex sequence",
			input: "B543260000001234",
			expected: Scores{
				RepeatLetterScore:     repeatScoreMap[6],
				IncreasingLetterScore: sequenceScoreMap[5],
				DecreasingLetterScore: sequenceScoreMap[4],
				UniqueLettersCount:    8,
			},
		},
		{
			name:  "complex sequence 2",
			input: "1234B54321000000",
			expected: Scores{
				RepeatLetterScore:     repeatScoreMap[6],
				IncreasingLetterScore: sequenceScoreMap[4],
				DecreasingLetterScore: sequenceScoreMap[6],
				UniqueLettersCount:    7,
			},
		},
		{
			name:  "wrapping sequence",
			input: "F0123",
			expected: Scores{
				IncreasingLetterScore: sequenceScoreMap[5],
				UniqueLettersCount:    5,
			},
		},
		{
			name:  "maximum length sequence",
			input: "0123456789ABCDEF",
			expected: Scores{
				IncreasingLetterScore: sequenceScoreMap[16],
				UniqueLettersCount:    16,
			},
		},
		{
			name:  "all unique characters",
			input: "0123456789ABCDEF",
			expected: Scores{
				IncreasingLetterScore: sequenceScoreMap[16],
				UniqueLettersCount:    16,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scores, err := CalculateScores(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, scores)
			}
		})
	}
}

func TestCharToValue(t *testing.T) {
	tests := []struct {
		input    byte
		expected int8
		valid    bool
	}{
		{'0', 0, true},
		{'9', 9, true},
		{'A', 10, true},
		{'F', 15, true},
		{'a', 10, true},
		{'f', 15, true},
		{'G', 0, false},
		{'!', 0, false},
		{0xFF, 0, false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("char_%c", tt.input), func(t *testing.T) {
			val, ok := charToValue(tt.input)
			assert.Equal(t, tt.valid, ok)
			if tt.valid {
				assert.Equal(t, tt.expected, val)
			}
		})
	}
}

func TestScoreMaps(t *testing.T) {
	// Test repeatScoreMap
	for i := minSeqLength; i <= maxSeqLength; i++ {
		t.Run(fmt.Sprintf("repeat_score_%d", i), func(t *testing.T) {
			score := repeatScoreMap[i]
			expected := int(math.Pow(float64(i), 1.5)) * 16
			assert.Equal(t, expected, score)
		})
	}

	// Test sequenceScoreMap
	for i := minSeqLength; i <= maxSeqLength; i++ {
		t.Run(fmt.Sprintf("sequence_score_%d", i), func(t *testing.T) {
			score := sequenceScoreMap[i]
			expected := int(math.Pow(float64(i-1), 1.5)) * 16
			assert.Equal(t, expected, score)
		})
	}
}

// 基准测试
func BenchmarkCalculateScores(b *testing.B) {
	benchmarks := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"single", "A"},
		{"short_repeat", "AAAA"},
		{"short_increase", "0123"},
		{"short_decrease", "FEDC"},
		{"magic", "49ABC"},
		{"mixed", "aAbBcC"},
		{"complex", "AABB1234CCDDEE"},
		{"max_length", "0123456789ABCDEF"},
		{"random_hex", "A1B2C3D4E5F60789"},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				scores, err := CalculateScores(bm.input)
				require.NoError(b, err)
				require.NotNil(b, scores)
			}
		})
	}
}

// 并发基准测试
func BenchmarkCalculateScores_Parallel(b *testing.B) {
	input := "0123456789ABCDEF"
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			scores, err := CalculateScores(input)
			require.NoError(b, err)
			require.NotNil(b, scores)
		}
	})
}

// 内存分配基准测试
func BenchmarkCalculateScores_Alloc(b *testing.B) {
	inputs := []string{
		"",
		"A",
		"AAAA",
		"0123",
		"FEDC",
		"49ABC",
		"0123456789ABCDEF",
	}

	for _, input := range inputs {
		b.Run(fmt.Sprintf("len_%d", len(input)), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				scores, err := CalculateScores(input)
				require.NoError(b, err)
				require.NotNil(b, scores)
			}
		})
	}
}

// 子基准测试：测试不同长度的输入
func BenchmarkCalculateScores_Lengths(b *testing.B) {
	lengths := []int{1, 4, 8, 12, 16}
	for _, length := range lengths {
		input := generateHexString(length)
		b.Run(fmt.Sprintf("len_%d", length), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				scores, err := CalculateScores(input)
				require.NoError(b, err)
				require.NotNil(b, scores)
			}
		})
	}
}

// 辅助函数：生成指定长度的十六进制字符串
func generateHexString(length int) string {
	const hexChars = "0123456789ABCDEF"
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		result[i] = hexChars[i%len(hexChars)]
	}
	return string(result)
}
