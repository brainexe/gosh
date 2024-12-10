package pkg

import "strings"

var ansiReset = "\033[0m"

func formatError(input error) string {
	return "\033[31m" + input.Error() + ansiReset
}

// getColorCode returns an ANSI color code based on the index
func getColorCode(idx int) string {
	start := "\u001B[1;" // with bold

	// todo use at least 8bit color support with pseudo random distribution

	colors := []string{
		"31m",       // Red
		"32m",       // Green
		"33m",       // Yellow
		"34m",       // Blue
		"35m",       // Magenta
		"36m",       // Cyan
		"37m",       // White
		"91m",       // Bright Red
		"92m",       // Bright Green
		"93m",       // Bright Yellow
		"94m",       // Bright Blue
		"95m",       // Bright Magenta
		"96m",       // Bright Cyan
		"97m",       // Bright White
		"38;5;208m", // Orange
		"38;5;201m", // Pink
		"38;5;120m", // Light Green
	}
	return start + colors[idx%len(colors)]
}

func maxHostName(hosts []string) int {
	maxLen := 0
	for _, host := range hosts {
		if len(host) > maxLen {
			maxLen = len(host)
		}
	}

	return maxLen
}

func getPadding(host string, maxLen int) string {
	return strings.Repeat(" ", maxLen-len(host)+1)
}
