package key

func isIncreasing(a, b byte) bool {
	return (a == '9' && b == 'A') || (a == 'F' && b == '0') || int(b) == int(a)+1
}

func isDecreasing(a, b byte) bool {
	return (a == 'A' && b == '9') || (a == '0' && b == 'F') || int(b) == int(a)-1
}

func countUniqueChars(s string) int {
	seen := make(map[rune]bool)
	for _, r := range s {
		seen[r] = true
	}
	return len(seen)
}
