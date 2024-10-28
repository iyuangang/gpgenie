package domain

import (
	"math"
	"strings"
)

// Scores 定义了分数结构体
type Scores struct {
	RepeatLetterScore     int
	IncreasingLetterScore int
	DecreasingLetterScore int
	MagicLetterScore      int
	UniqueLettersCount    int
}

// charToValueMap 是一个字符到数值的查找表，支持 '0'-'9', 'A'-'F', 'a'-'f'
var charToValueMap [128]int

// precomputedScores 包含预计算的分数字段，用于减少运行时计算开销
var (
	repeatScoreMap      [17]int
	increasingScoreMap  [17]int
	decreasingScoreMap  [17]int
	totalPossibleValues = 16
	magicSequence       = "49"
)

// 初始化查找表和预计算分数
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

	// 预计算分数，math.Pow(x, 1.5) * length (假设 length=16)
	// 这里预设长度为16，可以根据实际需求调整
	length := 16
	for x := 3; x <= 16; x++ {
		repeatScoreMap[x] = int(math.Pow(float64(x), 1.5)) * length
		increasingScoreMap[x] = int(math.Pow(float64(x-1), 1.5)) * length
		decreasingScoreMap[x] = int(math.Pow(float64(x-1), 1.5)) * length
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

	// 计算 b 是否是 a 的下一个字符，支持循环
	expected := (valA + 1) % totalPossibleValues
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

	// 计算 b 是否是 a 的上一个字符，支持循环
	expected := (valA - 1 + totalPossibleValues) % totalPossibleValues
	return valB == expected
}


// CalculateScores 计算给定字符串的各种分数
func CalculateScores(line string) (Scores, error) {

	line = strings.ToUpper(line) // 统一为大写

	var (
		repeatScore           int
		increasingScore       int
		decreasingScore       int
		magicScore            int
		uniqueLetters         [16]bool // 对应 '0'-'F'
		currentSequenceRepeat int      = 1
		currentSequenceInc    int      = 1
		currentSequenceDec    int      = 1
	)

	// 检查是否包含魔法序列 "49"
	if strings.Contains(line, magicSequence) {
		magicScore = -100
	}

	runes := []rune(line)
	length := len(runes)

	for i := 0; i < length; i++ {
		char := byte(runes[i])

		if val, ok := charToValue(char); ok {
			uniqueLetters[val] = true
		}

		if i > 0 {
			prevChar := byte(runes[i-1])

			// 重复字符评分
			if char == prevChar {
				currentSequenceRepeat++
			} else {
				if currentSequenceRepeat >= 3 {
					if currentSequenceRepeat < len(repeatScoreMap) {
						score := repeatScoreMap[currentSequenceRepeat]
						if score > repeatScore {
							repeatScore = score
						}
					} else {
						// 超出预计算范围，使用动态计算
						score := int(math.Pow(float64(currentSequenceRepeat), 1.5)) * length
						if score > repeatScore {
							repeatScore = score
						}
					}
				}
				currentSequenceRepeat = 1
			}

			// 递增评分
			if isIncreasing(prevChar, char) {
				currentSequenceInc++
			} else {
				if currentSequenceInc > 3 {
						score := increasingScoreMap[currentSequenceInc]
						if score > increasingScore {
							increasingScore = score
						}
				}
				currentSequenceInc = 1
			}

			// 递减评分
			if isDecreasing(prevChar, char) {
				currentSequenceDec++
			} else {
				if currentSequenceDec > 3 {
						score := decreasingScoreMap[currentSequenceDec]
						if score > decreasingScore {
							decreasingScore = score
						}
				}
				currentSequenceDec = 1
			}
		}
	}

	// 最后一次检查和评分计算
	if currentSequenceRepeat >= 3 {
			score := repeatScoreMap[currentSequenceRepeat]
			if score > repeatScore {
				repeatScore = score
			}
	}
	if currentSequenceInc > 3 {
			score := increasingScoreMap[currentSequenceInc]
			if score > increasingScore {
				increasingScore = score
			}
	}
	if currentSequenceDec > 3 {
			score := decreasingScoreMap[currentSequenceDec]
			if score > decreasingScore {
				decreasingScore = score
			}
	}

	// 计算唯一字母的数量
	uniqueCount := 0
	for _, present := range uniqueLetters {
		if present {
			uniqueCount++
		}
	}

	return Scores{
		RepeatLetterScore:     repeatScore,
		IncreasingLetterScore: increasingScore,
		DecreasingLetterScore: decreasingScore,
		MagicLetterScore:      magicScore,
		UniqueLettersCount:    uniqueCount,
	}, nil
}
