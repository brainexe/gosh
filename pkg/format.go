package pkg

import "fmt"

// Color codes for host prefixes (ANSI code fragments; full sequence built dynamically)
var Colors = []string{
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
	"38;5;208m", // Orange (256-color)
	"38;5;201m", // Pink (256-color)
	"38;5;120m", // Light Green (256-color)
}

const Reset = "\033[0m"

// FormatHost creates a colored/formatted host prefix
func FormatHost(host string, idx, maxLen int, noColor bool) string {
	padded := fmt.Sprintf("%-*s", maxLen, host)
	if noColor {
		return padded
	}
	code := Colors[idx%len(Colors)]
	return "\033[" + code + padded + Reset
}

// MaxLen returns the length of the longest string
func MaxLen(strings []string) int {
	maxLength := 0
	for _, s := range strings {
		if len(s) > maxLength {
			maxLength = len(s)
		}
	}
	return maxLength
}
