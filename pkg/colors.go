package pkg

// GetColorCode returns an ANSI color code based on the index
func GetColorCode(idx int) string {
	// todo proper lib, more colors when supported

	// ANSI color codes extended
	colors := []string{
		"\033[31m",       // Red
		"\033[32m",       // Green
		"\033[33m",       // Yellow
		"\033[34m",       // Blue
		"\033[35m",       // Magenta
		"\033[36m",       // Cyan
		"\033[37m",       // White
		"\033[91m",       // Bright Red
		"\033[92m",       // Bright Green
		"\033[93m",       // Bright Yellow
		"\033[94m",       // Bright Blue
		"\033[95m",       // Bright Magenta
		"\033[96m",       // Bright Cyan
		"\033[97m",       // Bright White
		"\033[38;5;208m", // Orange
		"\033[38;5;201m", // Pink
		"\033[38;5;120m", // Light Green
	}
	return colors[idx%len(colors)]
}
