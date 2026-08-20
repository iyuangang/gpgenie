package cmd

import (
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"

	"github.com/iyuangang/gpgenie/internal/key/vanity"
)

type vanityProgressDisplay struct {
	output io.Writer
	inline bool
	width  int
	active bool
}

func newVanityProgressDisplay(output io.Writer) *vanityProgressDisplay {
	return &vanityProgressDisplay{
		output: output,
		inline: writerIsTerminal(output),
	}
}

// Update redraws one terminal line. Redirected output keeps newline-delimited
// snapshots so logs remain readable and do not contain carriage-return frames.
func (d *vanityProgressDisplay) Update(line string, final bool) {
	if !d.inline {
		fmt.Fprintln(d.output, line)
		return
	}

	padding := d.width - len(line)
	if padding < 0 {
		padding = 0
	}
	fmt.Fprintf(d.output, "\r%s%s", line, strings.Repeat(" ", padding))
	d.width = len(line)
	d.active = true
	if final {
		fmt.Fprintln(d.output)
		d.width = 0
		d.active = false
	}
}

func (d *vanityProgressDisplay) Close() {
	if d.inline && d.active {
		fmt.Fprintln(d.output)
		d.width = 0
		d.active = false
	}
}

func writerIsTerminal(output io.Writer) bool {
	file, ok := output.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func formatVanityProgress(
	progress vanity.Progress,
	targetRun int,
	fallbackKeyID string,
	expectedAttempts float64,
) string {
	keyID := progress.BestKeyID
	if keyID == "" {
		keyID = fallbackKeyID
	}
	if keyID == "" {
		keyID = "-"
	}

	line := fmt.Sprintf(
		"vanity total=%s +%s %s best=%d/%d key=%s time=%s",
		formatVanityMetric(progress.Attempts),
		formatVanityMetric(progress.RunAttempts),
		formatVanityRate(progress.Rate),
		progress.BestRun,
		targetRun,
		keyID,
		progress.Elapsed.Round(time.Second),
	)
	if expectedAttempts > 0 {
		if progress.Rate > 0 {
			line += " eta~" + formatVanitySeconds(expectedAttempts/progress.Rate)
		} else {
			line += " eta=warming-up"
		}
	}
	return line
}

// expectedVanityAttempts returns the exact mean for suffix searches. Each
// candidate is independent, so completed attempts do not reduce the expected
// remaining wait. ScopeAny has overlapping start positions and is omitted
// instead of presenting a misleading estimate.
func expectedVanityAttempts(minRun int, scope vanity.Scope, digits vanity.DigitSet) float64 {
	if scope != vanity.ScopeSuffix || minRun < 1 || minRun > 16 {
		return 0
	}
	digitCount := len(digits.String())
	if digitCount == 0 {
		digitCount = 16
	}
	return math.Pow(16, float64(minRun)) / float64(digitCount)
}

func formatVanityMetric(value uint64) string {
	switch {
	case value >= 1_000_000_000_000_000_000:
		return fmt.Sprintf("%.3fE", float64(value)/1_000_000_000_000_000_000)
	case value >= 1_000_000_000_000_000:
		return fmt.Sprintf("%.3fP", float64(value)/1_000_000_000_000_000)
	case value >= 1_000_000_000_000:
		return fmt.Sprintf("%.3fT", float64(value)/1_000_000_000_000)
	case value >= 1_000_000_000:
		return fmt.Sprintf("%.3fB", float64(value)/1_000_000_000)
	case value >= 1_000_000:
		return fmt.Sprintf("%.3fM", float64(value)/1_000_000)
	case value >= 1_000:
		return fmt.Sprintf("%.3fK", float64(value)/1_000)
	default:
		return fmt.Sprintf("%d", value)
	}
}

func formatVanityRate(rate float64) string {
	if rate <= 0 {
		return "warming-up"
	}
	return formatVanityMetric(uint64(rate)) + "/s"
}

func formatVanitySeconds(seconds float64) string {
	const (
		minute = 60
		hour   = 60 * minute
		day    = 24 * hour
		year   = 365.25 * day
	)
	switch {
	case math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds < 0:
		return "unknown"
	case seconds < minute:
		return fmt.Sprintf("%.0fs", seconds)
	case seconds < hour:
		return fmt.Sprintf("%.1fm", seconds/minute)
	case seconds < day:
		return fmt.Sprintf("%.1fh", seconds/hour)
	case seconds < year:
		return fmt.Sprintf("%.1fd", seconds/day)
	default:
		return fmt.Sprintf("%.1fy", seconds/year)
	}
}
