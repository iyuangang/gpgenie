package key

import (
	"fmt"
	"gpgenie/internal/repository"
	"math"
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
	// 执行各项分析
	if err := a.analyzeScores(); err != nil {
		return err
	}
	if err := a.analyzeUniqueLettersCount(); err != nil {
		return err
	}
	if err := a.analyzeScoreComponents(); err != nil {
		return err
	}
	if err := a.analyzeCorrelation(); err != nil {
		return err
	}

	// 可以在此添加更多的分析维度

	return nil
}

// analyzeScores 分析 Score 的分布情况
func (a *Analyzer) analyzeScores() error {
	stats, err := a.repo.GetScoreStatistics()
	if err != nil {
		return fmt.Errorf("failed to get score statistics: %w", err)
	}

	fmt.Println("=== Score Analysis ===")
	fmt.Printf("Total Keys: %d\n", stats.Count)
	fmt.Printf("Average Score: %.2f\n", stats.Average)
	fmt.Printf("Minimum Score: %.2f\n", stats.Min)
	fmt.Printf("Maximum Score: %.2f\n", stats.Max)
	fmt.Println()
	return nil
}

// analyzeUniqueLettersCount 分析 UniqueLettersCount 的分布情况
func (a *Analyzer) analyzeUniqueLettersCount() error {
	stats, err := a.repo.GetUniqueLettersStatistics()
	if err != nil {
		return fmt.Errorf("failed to get unique letters count statistics: %w", err)
	}

	fmt.Println("=== Unique Letters Count Analysis ===")
	fmt.Printf("Total Keys: %d\n", stats.Count)
	fmt.Printf("Average Unique Letters Count: %.2f\n", stats.Average)
	fmt.Printf("Minimum Unique Letters Count: %.2f\n", stats.Min)
	fmt.Printf("Maximum Unique Letters Count: %.2f\n", stats.Max)
	fmt.Println()
	return nil
}

// analyzeScoreComponents 分析各个分数组成的情况
func (a *Analyzer) analyzeScoreComponents() error {
	stats, err := a.repo.GetScoreComponentsStatistics()
	if err != nil {
		return fmt.Errorf("failed to get score components statistics: %w", err)
	}

	fmt.Println("=== Score Components Analysis ===")
	fmt.Printf("Average Repeat Letter Score: %.2f\n", stats.AverageRepeat)
	fmt.Printf("Average Increasing Letter Score: %.2f\n", stats.AverageIncreasing)
	fmt.Printf("Average Decreasing Letter Score: %.2f\n", stats.AverageDecreasing)
	fmt.Printf("Average Magic Letter Score: %.2f\n", stats.AverageMagic)
	fmt.Println()
	return nil
}

// analyzeCorrelation 分析 Score 与 UniqueLettersCount 的相关性
func (a *Analyzer) analyzeCorrelation() error {
	correlation, err := a.repo.GetCorrelationCoefficient()
	if err != nil {
		return fmt.Errorf("failed to calculate correlation coefficient: %w", err)
	}

	fmt.Println("=== Correlation Analysis ===")
	fmt.Printf("Pearson Correlation Coefficient between Score and Unique Letters Count: %.4f\n", correlation)
	switch {
	case math.Abs(correlation) > 0.7:
		fmt.Println("Interpretation: Strong correlation detected.")
	case math.Abs(correlation) > 0.4:
		fmt.Println("Interpretation: Moderate correlation detected.")
	default:
		fmt.Println("Interpretation: Weak or no correlation detected.")
	}
	fmt.Println()
	return nil
}
