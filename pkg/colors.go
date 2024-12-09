package pkg

// GetColorCode returns an ANSI color code based on the index
func GetColorCode(idx int) string {
	// todo more colors?
	// ANSI color codes with bold
	colors := []string{
		"\033[1;31m",       // Bold Red
		"\033[1;32m",       // Bold Green
		"\033[1;33m",       // Bold Yellow
		"\033[1;34m",       // Bold Blue
		"\033[1;35m",       // Bold Magenta
		"\033[1;36m",       // Bold Cyan
		"\033[1;37m",       // Bold White
		"\033[1;91m",       // Bold Bright Red
		"\033[1;92m",       // Bold Bright Green
		"\033[1;93m",       // Bold Bright Yellow
		"\033[1;94m",       // Bold Bright Blue
		"\033[1;95m",       // Bold Bright Magenta
		"\033[1;96m",       // Bold Bright Cyan
		"\033[1;97m",       // Bold Bright White
		"\033[1;38;5;208m", // Bold Orange
		"\033[1;38;5;201m", // Bold Pink
		"\033[1;38;5;120m", // Bold Light Green
	}
	return colors[idx%len(colors)]
}
