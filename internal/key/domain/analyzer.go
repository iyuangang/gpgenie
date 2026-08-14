package domain

import (
	"fmt"

	"github.com/iyuangang/gpgenie/internal/repository"
)

type Analyzer struct {
	repo repository.KeyRepository
}

func NewAnalyzer(repo repository.KeyRepository) *Analyzer {
	return &Analyzer{repo: repo}
}

func (a *Analyzer) PerformAnalysis() error {
	stats, err := a.repo.GetAnalysisStats()
	if err != nil {
		return fmt.Errorf("failed to get analysis statistics: %w", err)
	}

	fmt.Println("=== Score Analysis ===")
	fmt.Printf("Total Keys: %d\n", stats.Score.Count)
	fmt.Printf("Average Score: %.2f\n", stats.Score.Average)
	fmt.Printf("Minimum Score: %.2f\n", stats.Score.Min)
	fmt.Printf("Maximum Score: %.2f\n", stats.Score.Max)
	fmt.Println()

	fmt.Println("=== Unique Letters Count Analysis ===")
	fmt.Printf("Total Keys: %d\n", stats.UniqueLetters.Count)
	fmt.Printf("Average Unique Letters Count: %.2f\n", stats.UniqueLetters.Average)
	fmt.Printf("Minimum Unique Letters Count: %.2f\n", stats.UniqueLetters.Min)
	fmt.Printf("Maximum Unique Letters Count: %.2f\n", stats.UniqueLetters.Max)
	fmt.Println()

	fmt.Println("=== Score Components Analysis ===")
	fmt.Printf("Average Repeat Letter Score: %.2f\n", stats.Components.AverageRepeat)
	fmt.Printf("Average Increasing Letter Score: %.2f\n", stats.Components.AverageIncreasing)
	fmt.Printf("Average Decreasing Letter Score: %.2f\n", stats.Components.AverageDecreasing)
	fmt.Printf("Average Magic Letter Score: %.2f\n", stats.Components.AverageMagic)
	fmt.Println()

	fmt.Println("=== Correlation Analysis ===")
	fmt.Printf("Pearson Correlation Coefficient between Score and Unique Letters Count: %.4f\n", stats.Correlation)
	switch {
	case stats.Correlation > 0.7 || stats.Correlation < -0.7:
		fmt.Println("Interpretation: Strong correlation detected.")
	case stats.Correlation > 0.4 || stats.Correlation < -0.4:
		fmt.Println("Interpretation: Moderate correlation detected.")
	default:
		fmt.Println("Interpretation: Weak or no correlation detected.")
	}
	fmt.Println()
	return nil
}
