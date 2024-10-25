package domain

import (
	"fmt"

	"gpgenie/internal/repository"
)

type Analyzer struct {
	repo repository.KeyRepository
}

func NewAnalyzer(repo repository.KeyRepository) *Analyzer {
	return &Analyzer{repo: repo}
}

func (a *Analyzer) PerformAnalysis() error {
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

	return nil
}

func (a *Analyzer) analyzeScores() error {
	stats, err := a.repo.GetScoreStats()
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

func (a *Analyzer) analyzeUniqueLettersCount() error {
	stats, err := a.repo.GetUniqueLettersStats()
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

func (a *Analyzer) analyzeScoreComponents() error {
	stats, err := a.repo.GetScoreComponentsStats()
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

func (a *Analyzer) analyzeCorrelation() error {
	correlation, err := a.repo.GetCorrelationCoefficient()
	if err != nil {
		return fmt.Errorf("failed to calculate correlation coefficient: %w", err)
	}

	fmt.Println("=== Correlation Analysis ===")
	fmt.Printf("Pearson Correlation Coefficient between Score and Unique Letters Count: %.4f\n", correlation)
	switch {
	case correlation > 0.7 || correlation < -0.7:
		fmt.Println("Interpretation: Strong correlation detected.")
	case correlation > 0.4 || correlation < -0.4:
		fmt.Println("Interpretation: Moderate correlation detected.")
	default:
		fmt.Println("Interpretation: Weak or no correlation detected.")
	}
	fmt.Println()
	return nil
}
