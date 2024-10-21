package key

import (
	"fmt"
	"math"
	"os"
	"text/tabwriter"

	"gpgenie/internal/key/models"
	"gpgenie/internal/repository"
)

// Analyzer 负责执行数据分析
type Analyzer struct {
	repo repository.KeyRepository
}

// NewAnalyzer 创建一个新的 Analyzer 实例
func NewAnalyzer(repo repository.KeyRepository) *Analyzer {
	return &Analyzer{
		repo: repo,
	}
}

// PerformAnalysis 执行多维度的数据分析
func (a *Analyzer) PerformAnalysis() error {
	// 获取所有 KeyInfo 数据
	keys, err := a.repo.GetAllKeys()
	if err != nil {
		return fmt.Errorf("failed to retrieve keys: %w", err)
	}

	if len(keys) == 0 {
		fmt.Println("No keys found in the database for analysis.")
		return nil
	}

	// 执行各项分析
	a.analyzeScores(keys)
	a.analyzeUniqueLettersCount(keys)
	a.analyzeScoreComponents(keys)
	a.analyzeCorrelation(keys)
	a.analyzeHistogram(keys)

	return nil
}

// analyzeScores 分析 Score 的分布情况
func (a *Analyzer) analyzeScores(keys []models.KeyInfo) {
	var total, min, max float64
	min = math.MaxFloat64
	max = 0

	for _, key := range keys {
		score := float64(key.Score)
		total += score
		if score < min {
			min = score
		}
		if score > max {
			max = score
		}
	}

	average := total / float64(len(keys))

	fmt.Println("=== Score Analysis ===")
	fmt.Printf("Total Keys: %d\n", len(keys))
	fmt.Printf("Average Score: %.2f\n", average)
	fmt.Printf("Minimum Score: %.2f\n", min)
	fmt.Printf("Maximum Score: %.2f\n", max)
	fmt.Println()
}

// analyzeUniqueLettersCount 分析 UniqueLettersCount 的分布情况
func (a *Analyzer) analyzeUniqueLettersCount(keys []models.KeyInfo) {
	var total, min, max float64
	min = math.MaxFloat64
	max = 0

	for _, key := range keys {
		count := float64(key.UniqueLettersCount)
		total += count
		if count < min {
			min = count
		}
		if count > max {
			max = count
		}
	}

	average := total / float64(len(keys))

	fmt.Println("=== Unique Letters Count Analysis ===")
	fmt.Printf("Total Keys: %d\n", len(keys))
	fmt.Printf("Average Unique Letters Count: %.2f\n", average)
	fmt.Printf("Minimum Unique Letters Count: %.2f\n", min)
	fmt.Printf("Maximum Unique Letters Count: %.2f\n", max)
	fmt.Println()
}

// analyzeScoreComponents 分析各个分数组成的情况
func (a *Analyzer) analyzeScoreComponents(keys []models.KeyInfo) {
	var totalRepeat, totalIncreasing, totalDecreasing, totalMagic float64
	for _, key := range keys {
		totalRepeat += float64(key.RepeatLetterScore)
		totalIncreasing += float64(key.IncreasingLetterScore)
		totalDecreasing += float64(key.DecreasingLetterScore)
		totalMagic += float64(key.MagicLetterScore)
	}

	averageRepeat := totalRepeat / float64(len(keys))
	averageIncreasing := totalIncreasing / float64(len(keys))
	averageDecreasing := totalDecreasing / float64(len(keys))
	averageMagic := totalMagic / float64(len(keys))

	fmt.Println("=== Score Components Analysis ===")
	fmt.Printf("Average Repeat Letter Score: %.2f\n", averageRepeat)
	fmt.Printf("Average Increasing Letter Score: %.2f\n", averageIncreasing)
	fmt.Printf("Average Decreasing Letter Score: %.2f\n", averageDecreasing)
	fmt.Printf("Average Magic Letter Score: %.2f\n", averageMagic)
	fmt.Println()
}

// analyzeCorrelation 分析 Score 与 UniqueLettersCount 的相关性
func (a *Analyzer) analyzeCorrelation(keys []models.KeyInfo) {
	var sumX, sumY, sumXY, sumX2, sumY2 float64
	n := float64(len(keys))

	for _, key := range keys {
		x := float64(key.Score)
		y := float64(key.UniqueLettersCount)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
		sumY2 += y * y
	}

	// 计算 Pearson 相关系数
	numerator := (n * sumXY) - (sumX * sumY)
	denominator := math.Sqrt((n*sumX2 - sumX*sumX) * (n*sumY2 - sumY*sumY))

	var correlation float64
	if denominator != 0 {
		correlation = numerator / denominator
	} else {
		correlation = 0
	}

	fmt.Println("=== Correlation Analysis ===")
	fmt.Printf("Pearson Correlation Coefficient between Score and Unique Letters Count: %.4f\n", correlation)
	fmt.Println()
}

// analyzeHistogram 分析 Score 的直方图
func (a *Analyzer) analyzeHistogram(keys []models.KeyInfo) {
	// 定义直方图的区间
	bins := []int{-100, 0, 100, 200, 300, 400, 500, 600, 700}
	counts := make([]int, len(bins)-1)

	// 统计每个区间的数量
	for _, key := range keys {
		for i := 0; i < len(bins)-1; i++ {
			if key.Score >= bins[i] && key.Score < bins[i+1] {
				counts[i]++
				break
			}
		}
	}

	// 打印直方图
	fmt.Println("=== Score Histogram ===")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Range\tCount\n")
	for i := 0; i < len(bins)-1; i++ {
		fmt.Fprintf(w, "%d - %d\t%d\n", bins[i], bins[i+1]-1, counts[i])
	}
	w.Flush()
	fmt.Println()
}
