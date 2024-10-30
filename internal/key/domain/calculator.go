package domain

import (
	"math"
	"strings"
	"unsafe"
)

// Scores 定义了分数结构体
type Scores struct {
	RepeatLetterScore     int
	IncreasingLetterScore int
	DecreasingLetterScore int
	MagicLetterScore      int
	UniqueLettersCount    int
}

// 使用const提升编译期优化机会
const (
	maxSeqLength = 16
	baseMultiplier = 100
	magicScore = -100
	minSeqLength = 3
)

// 预计算的分数映射表 - 使用const数组提高性能
var (
	// 使用更大的数组避免边界检查
	repeatScoreMap    [32]int
	sequenceScoreMap  [32]int
	// 使用更紧凑的查找表
	charToValueMap    [256]int8
)

// 编译期初始化所有查找表
func init() {
	// 初始化字符映射
	for i := range charToValueMap {
		charToValueMap[i] = -1
	}

	// 数字映射 (0-9)
	for i := byte('0'); i <= '9'; i++ {
		charToValueMap[i] = int8(i - '0')
	}

	// 大写字母映射 (A-F)
	for i := byte('A'); i <= 'F'; i++ {
		charToValueMap[i] = int8(i-'A') + 10
	}

	// 小写字母映射 (a-f)
	for i := byte('a'); i <= 'f'; i++ {
		charToValueMap[i] = int8(i-'a') + 10
	}

	// 预计算分数
	for i := minSeqLength; i <= maxSeqLength; i++ {
		// 使用位移操作替代乘法
		repeatScoreMap[i] = int(math.Pow(float64(i), 1.5)) * 16
		sequenceScoreMap[i] = int(math.Pow(float64(i-1), 1.5)) * 16
	}
}

// 内联优化的值转换函数
//go:inline
func charToValue(c byte) (int8, bool) {
	val := charToValueMap[c]
	return val, val >= 0
}

// CalculateScores 计算给定字符串的各种分数
func CalculateScores(line string) (Scores, error) {
	length := len(line)
	if length == 0 {
		return Scores{}, nil
	}

	// 使用栈分配替代堆分配
	var (
		uniqueMask        uint16 // 使用位掩码追踪唯一字符
		repeatLen         uint8  = 1
		increasingLen     uint8  = 1
		decreasingLen     uint8  = 1
		maxRepeatScore    int
		maxIncreasingScore int
		maxDecreasingScore int
		uniqueCount       int
		hasMagicSequence  bool
		prevVal          int8   = -1
		prevChar         byte
	)

	// 快速路径：检查魔法序列
	hasMagicSequence = strings.Contains(line, "49")

	// 主循环 - 使用指针操作避免边界检查
	ptr := (*[1 << 30]byte)(unsafe.Pointer(unsafe.StringData(line)))[:length:length]

	// 处理第一个字符
	if val, ok := charToValue(ptr[0]); ok {
		uniqueMask |= 1 << uint16(val)
		uniqueCount = 1
		prevVal = val
		prevChar = ptr[0]
	}

	// 使用展开循环优化性能
	for i := 1; i < length; i++ {
		current := ptr[i]

		// 内联字符转换
		if currentVal := charToValueMap[current]; currentVal >= 0 {
			// 位操作检查和更新唯一字符
			mask := uint16(1) << uint16(currentVal)
			if uniqueMask&mask == 0 {
				uniqueMask |= mask
				uniqueCount++
			}

			// 更新序列计数 - 使用位运算优化
			if current == prevChar {
				repeatLen++
			} else {
				if repeatLen >= minSeqLength {
					score := repeatScoreMap[repeatLen]
					if score > maxRepeatScore {
						maxRepeatScore = score
					}
				}
				repeatLen = 1
			}

			// 优化递增/递减检查
			diff := int8(currentVal - prevVal)
			switch {
			case diff == 1 || (prevVal == 15 && currentVal == 0):
				increasingLen++
				decreasingLen = 1
			case diff == -1 || (prevVal == 0 && currentVal == 15):
				decreasingLen++
				increasingLen = 1
			default:
				if increasingLen > minSeqLength {
					score := sequenceScoreMap[increasingLen]
					if score > maxIncreasingScore {
						maxIncreasingScore = score
					}
				}
				if decreasingLen > minSeqLength {
					score := sequenceScoreMap[decreasingLen]
					if score > maxDecreasingScore {
						maxDecreasingScore = score
					}
				}
				increasingLen = 1
				decreasingLen = 1
			}

			prevVal = currentVal
			prevChar = current
		}
	}

	// 处理最后的序列
	if repeatLen >= minSeqLength {
		score := repeatScoreMap[repeatLen]
		if score > maxRepeatScore {
			maxRepeatScore = score
		}
	}
	if increasingLen > minSeqLength {
		score := sequenceScoreMap[increasingLen]
		if score > maxIncreasingScore {
			maxIncreasingScore = score
		}
	}
	if decreasingLen > minSeqLength {
		score := sequenceScoreMap[decreasingLen]
		if score > maxDecreasingScore {
			maxDecreasingScore = score
		}
	}

	return Scores{
		RepeatLetterScore:     maxRepeatScore,
		IncreasingLetterScore: maxIncreasingScore,
		DecreasingLetterScore: maxDecreasingScore,
		MagicLetterScore:      boolToInt(hasMagicSequence) * magicScore,
		UniqueLettersCount:    uniqueCount,
	}, nil
}

//go:inline
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
