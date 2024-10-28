package domain

import (
	"testing"
)

func TestIsIncreasing(t *testing.T) {
	tests := []struct {
		a        byte
		b        byte
		expected bool
	}{
		{'0', '1', true},
		{'9', 'A', true},
		{'F', '0', true},
		{'A', 'B', true},
		{'C', 'C', false},
		{'F', '1', false},
		{'g', 'h', false}, // 非法字符
		{'5', '6', true},
		{'E', 'F', true},
	}

	for _, test := range tests {
		result := isIncreasing(test.a, test.b)
		if result != test.expected {
			t.Errorf("isIncreasing(%c, %c) = %v; want %v", test.a, test.b, result, test.expected)
		}
	}
}

func TestIsDecreasing(t *testing.T) {
	tests := []struct {
		a        byte
		b        byte
		expected bool
	}{
		{'1', '0', true},
		{'A', '9', true},
		{'0', 'F', true},
		{'B', 'A', true},
		{'C', 'C', false},
		{'0', '1', false},
		{'g', 'f', false}, // 非法字符
		{'6', '5', true},
		{'F', 'E', true},
	}

	for _, test := range tests {
		result := isDecreasing(test.a, test.b)
		if result != test.expected {
			t.Errorf("isDecreasing(%c, %c) = %v; want %v", test.a, test.b, result, test.expected)
		}
	}
}

func TestCalculateScores(t *testing.T) {
	tests := []struct {
		input    string
		expected Scores
	}{
		{"8888888888888888", Scores{1024, 0, 0, 0, 1}},
		{"0123456789ABCDEF", Scores{0, 928, 0, 0, 16}},
		{"FEDCBA9876543210", Scores{0, 0, 928, 0, 16}},
		{"0123456666FEDCBA", Scores{128, 224, 176, 0, 13}},
		{"1929394959697989", Scores{0, 0, 0, -100, 9}},
		{"FCEC7777789ABCDF", Scores{176, 224, 0, 0, 9}},
		{"42E42EE22E4EE4E2", Scores{0, 0, 0, 0, 3}},
	}

	for _, test := range tests {
		result, err := CalculateScores(test.input)
		if err != nil {
			t.Errorf("CalculateScores(%q) returned error: %v", test.input, err)
			continue
		}
		if result != test.expected {
			t.Errorf("CalculateScores(%q) = %v; want %v", test.input, result, test.expected)
		}
	}
}
