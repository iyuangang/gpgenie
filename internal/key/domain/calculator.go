package domain

import (
	"math"
	"strings"
)

// charToValueMap 是一个字符到数值的查找表，支持 '0'-'9', 'A'-'F', 'a'-'f'
var charToValueMap [128]int

func init() {
	// 初始化查找表，未定义的字符对应 -1
	for i := range charToValueMap {
		charToValueMap[i] = -1
	}
	// '0' - '9' -> 0 - 9
	for c := byte('0'); c <= '9'; c++ {
		charToValueMap[c] = int(c - '0')
	}
	// 'A' - 'F' -> 10 - 15
	for c := byte('A'); c <= 'F'; c++ {
		charToValueMap[c] = int(c-'A') + 10
	}
	// 'a' - 'f' -> 10 - 15
	for c := byte('a'); c <= 'f'; c++ {
		charToValueMap[c] = int(c-'a') + 10
	}
}

// charToValue 将十六进制字符转换为对应的数值 (0-15)
// 支持大写和小写字母
func charToValue(c byte) (int, bool) {
	if c >= 128 {
		return 0, false
	}
	val := charToValueMap[c]
	if val == -1 {
		return 0, false
	}
	return val, true
}

// isIncreasing 判断字符 b 是否是字符 a 的递增字符，支持循环逻辑
func isIncreasing(a, b byte) bool {
	valA, okA := charToValue(a)
	valB, okB := charToValue(b)
	if !okA || !okB {
		// 非法字符
		return false
	}

	// 总共有16个可能值 (0-15)
	total := 16

	// 计算 b 是否是 a 的下一个字符，支持循环
	expected := (valA + 1) % total
	return valB == expected
}

// isDecreasing 判断字符 b 是否是字符 a 的递减字符，支持循环逻辑
func isDecreasing(a, b byte) bool {
	valA, okA := charToValue(a)
	valB, okB := charToValue(b)
	if !okA || !okB {
		// 非法字符
		return false
	}

	// 总共有16个可能值 (0-15)
	total := 16

	// 计算 b 是否是 a 的上一个字符，支持循环
	expected := (valA - 1 + total) % total
	return valB == expected
}

// Scores 定义了分数结构体
type Scores struct {
	RepeatLetterScore      int
	IncreasingLetterScore int
	DecreasingLetterScore int
	MagicLetterScore       int
	UniqueLettersCount     int
}

// CalculateScores 计算给定字符串的各种分数
func CalculateScores(line string) Scores {
	line = strings.ToUpper(line) // 转换为大写以统一处理

	var repeatScore, increasingScore, decreasingScore, magicScore int
	uniqueLetters := make(map[rune]struct{})

	currentSequenceRepeat := 1
	currentSequenceInc := 1
	currentSequenceDec := 1

	// 检查魔法字符 "49"
	if strings.Contains(line, "49") {
		magicScore = -100
	}

	runes := []rune(line)
	length := len(runes)

	for i := 0; i < length; i++ {
		char := runes[i]
		uniqueLetters[char] = struct{}{}

		if i > 0 {
			prevChar := runes[i-1]

			// 重复字符评分
			if char == prevChar {
				currentSequenceRepeat++
			} else {
				if currentSequenceRepeat >= 3 {
					score := int(math.Pow(float64(currentSequenceRepeat), 1.5)) * length
					if score > repeatScore {
						repeatScore = score
					}
				}
				currentSequenceRepeat = 1
			}

			// 递增评分
			if isIncreasing(byte(prevChar), byte(char)) {
				currentSequenceInc++
			} else {
				if currentSequenceInc > 3 {
					score := int(math.Pow(float64(currentSequenceInc-1), 1.5)) * length
					if score > increasingScore {
						increasingScore = score
					}
				}
				currentSequenceInc = 1
			}

			// 递减评分
			if isDecreasing(byte(prevChar), byte(char)) {
				currentSequenceDec++
			} else {
				if currentSequenceDec > 3 {
					score := int(math.Pow(float64(currentSequenceDec-1), 1.5)) * length
					if score > decreasingScore {
						decreasingScore = score
					}
				}
				currentSequenceDec = 1
			}
		}
	}

	// 最后一次检查
	if currentSequenceRepeat >= 3 {
		score := int(math.Pow(float64(currentSequenceRepeat), 1.5)) * length
		if score > repeatScore {
			repeatScore = score
		}
	}
	if currentSequenceInc > 3 {
		score := int(math.Pow(float64(currentSequenceInc-1), 1.5)) * length
		if score > increasingScore {
			increasingScore = score
		}
	}
	if currentSequenceDec > 3 {
		score := int(math.Pow(float64(currentSequenceDec-1), 1.5)) * length
		if score > decreasingScore {
			decreasingScore = score
		}
	}

	return Scores{
		RepeatLetterScore:      repeatScore,
		IncreasingLetterScore: increasingScore,
		DecreasingLetterScore: decreasingScore,
		MagicLetterScore:       magicScore,
		UniqueLettersCount:     len(uniqueLetters),
	}
}
