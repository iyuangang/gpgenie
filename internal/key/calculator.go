package key

import (
	"math"
	"strings"
)

type Scores struct {
    RLScore      int
    ILScore      int
    DLScore      int
    MLScore      int
    LettersCount int
}

func calculateScores(line string) Scores {
	line = strings.ToUpper(line) // Convert to uppercase for consistency

	rlScore := calculateRepeatScore(line)
	ilScore := calculateIncreasingScore(line)
	dlScore := calculateDecreasingScore(line)
	mlScore := calculateMLScore(line)

	return Scores{
		RLScore:      rlScore,
		ILScore:      ilScore,
		DLScore:      dlScore,
		MLScore:      mlScore,
		LettersCount: countUniqueChars(line),
	}
}

func calculateRepeatScore(s string) int {
	maxScore := 0
	for i := 0; i < len(s); i++ {
		count := 1
		for j := i + 1; j < len(s); j++ {
			if s[j] == s[i] {
				count++
			} else {
				break
			}
		}
		if count >= 3 {
			score := int(math.Pow(float64(count), 1.5)) * len(s)
			if score > maxScore {
				maxScore = score
			}
		}
		i += count - 1
	}
	return maxScore
}

func calculateIncreasingScore(s string) int {
	return calculateSequenceScore(s, true)
}

func calculateDecreasingScore(s string) int {
	return calculateSequenceScore(s, false)
}

func calculateSequenceScore(s string, increasing bool) int {
	maxScore := 0
	currentSequence := 1
	for i := 1; i < len(s); i++ {
		if (increasing && isIncreasing(s[i-1], s[i])) || (!increasing && isDecreasing(s[i-1], s[i])) {
			currentSequence++
		} else {
			if currentSequence > 3 {
				score := int(math.Pow(float64(currentSequence - 1), 1.5)) * len(s)
				if score > maxScore {
					maxScore = score
				}
			}
			currentSequence = 1
		}
	}
	if currentSequence > 3 {
		score := int(math.Pow(float64(currentSequence - 1), 1.5)) * len(s)
		if score > maxScore {
			maxScore = score
		}
	}
	return maxScore
}

func calculateMLScore(s string) int {
	if strings.Contains(s, "49") {
		return -100
	}
	return 0
}
