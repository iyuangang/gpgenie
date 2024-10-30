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

// 预计算的分数映射表
var (
	repeatScoreMap    [17]int
	sequenceScoreMap  [17]int
	charToValueMap    [128]int
	valueInitialized  = false
)

// 初始化所有查找表
func init() {
	if !valueInitialized {
		initCharToValueMap()
		initScoreMaps()
		valueInitialized = true
	}
}

// 初始化字符到值的映射
func initCharToValueMap() {
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

// 初始化分数映射表
func initScoreMaps() {
	// 计算重复字符分数
	for x := 3; x <= 16; x++ {
		repeatScoreMap[x] = int(math.Pow(float64(x), 1.5)) * 16
	}

	// 计算序列分数
	for x := 4; x <= 16; x++ {
		sequenceScoreMap[x] = int(math.Pow(float64(x-1), 1.5)) * 16
	}
}

// charToValue 将十六进制字符转换为对应的数值
func charToValue(c byte) (int, bool) {
	if c >= 128 {
		return 0, false
	}
	val := charToValueMap[c]
	return val, val != -1
}

// CalculateScores 计算给定字符串的各种分数
func CalculateScores(line string) (Scores, error) {
	//line = strings.ToUpper(line)
	length := len(line)
	//if length == 0 {
	//	return Scores{}, nil
	//}

	var (
		uniqueLetters      [16]bool
		repeatLen         = 1
		increasingLen     = 1
		decreasingLen     = 1
		maxRepeatScore    = 0
		maxIncreasingScore = 0
		maxDecreasingScore = 0
		uniqueCount       = 0
	)

	// 处理第一个字符
	if val, ok := charToValue(line[0]); ok {
		uniqueLetters[val] = true
		uniqueCount = 1
	}

	// 处理剩余字符
	for i := 1; i < length; i++ {
		current := line[i]
		prev := line[i-1]

		// 更新唯一字符计数
		if val, ok := charToValue(current); ok && !uniqueLetters[val] {
			uniqueLetters[val] = true
			uniqueCount++
		}

		// 更新重复序列
		if current == prev {
			repeatLen++
		} else {
			if repeatLen >= 3 {
				score := repeatScoreMap[repeatLen]
				if score > maxRepeatScore {
					maxRepeatScore = score
				}
			}
			repeatLen = 1
		}

		// 更新递增序列
		if isIncreasing(prev, current) {
			increasingLen++
		} else {
			if increasingLen > 3 {
				score := sequenceScoreMap[increasingLen]
				if score > maxIncreasingScore {
					maxIncreasingScore = score
				}
			}
			increasingLen = 1
		}

		// 更新递减序列
		if isDecreasing(prev, current) {
			decreasingLen++
		} else {
			if decreasingLen > 3 {
				score := sequenceScoreMap[decreasingLen]
				if score > maxDecreasingScore {
					maxDecreasingScore = score
				}
			}
			decreasingLen = 1
		}
	}

	// 处理最后的序列
	if repeatLen >= 3 {
		score := repeatScoreMap[repeatLen]
		if score > maxRepeatScore {
			maxRepeatScore = score
		}
	}
	if increasingLen > 3 {
		score := sequenceScoreMap[increasingLen]
		if score > maxIncreasingScore {
			maxIncreasingScore = score
		}
	}
	if decreasingLen > 3 {
		score := sequenceScoreMap[decreasingLen]
		if score > maxDecreasingScore {
			maxDecreasingScore = score
		}
	}

	scores := Scores{
		RepeatLetterScore:     maxRepeatScore,
		IncreasingLetterScore: maxIncreasingScore,
		DecreasingLetterScore: maxDecreasingScore,
		UniqueLettersCount:    uniqueCount,
	}

	// 检查魔法序列
	if strings.Contains(line, "49") {
		scores.MagicLetterScore = -100
	}

	return scores, nil
}

// isIncreasing 判断字符 b 是否是字符 a 的递增字符
func isIncreasing(a, b byte) bool {
	valA, okA := charToValue(a)
	valB, okB := charToValue(b)
	if !okA || !okB {
		return false
	}
	return valB == (valA+1)%16
}

// isDecreasing 判断字符 b 是否是字符 a 的递减字符
func isDecreasing(a, b byte) bool {
	valA, okA := charToValue(a)
	valB, okB := charToValue(b)
	if !okA || !okB {
		return false
	}
	return valB == (valA-1+16)%16
}
