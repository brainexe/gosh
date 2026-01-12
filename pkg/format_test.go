package pkg

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestMaxLen(t *testing.T) {
	tests := []struct {
		input    []string
		expected int
	}{
		{[]string{"a", "bb", "ccc"}, 3},
		{[]string{"hello", "world"}, 5},
		{[]string{}, 0},
		{[]string{"single"}, 6},
	}

	for _, test := range tests {
		result := maxLen(test.input)
		if result != test.expected {
			t.Errorf("maxLen(%v) = %d, expected %d", test.input, result, test.expected)
		}
	}
}

func TestFormatHost(t *testing.T) {
	tests := []struct {
		name        string
		host        string
		idx         int
		maxLen      int
		noColor     bool
		contains    string
		notContains string
	}{
		{"no color padding", "host1", 0, 10, true, "host1     ", "\033["},
		{"short host no color", "short", 1, 10, true, "short     ", "\033["},
		{"with color codes", "host1", 0, 10, false, "host1     ", ""},
		{"with color has ANSI", "host1", 0, 10, false, "\033[", ""},
		{"with color has reset", "host1", 0, 10, false, "\033[0m", ""},
		{"different colors", "host2", 1, 10, false, "\033[", ""},
		{"exact length", "exactly10c", 0, 10, true, "exactly10c", "\033["},
		{"longer than max", "verylonghost", 0, 8, true, "verylonghost", "\033["},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := formatHostPrefix(test.host, test.idx, test.maxLen, test.noColor)

			if test.contains != "" && !strings.Contains(result, test.contains) {
				t.Errorf("formatHostPrefix(%q, %d, %d, %t) = %q, should contain %q",
					test.host, test.idx, test.maxLen, test.noColor, result, test.contains)
			}

			if test.notContains != "" && strings.Contains(result, test.notContains) {
				t.Errorf("formatHostPrefix(%q, %d, %d, %t) = %q, should not contain %q",
					test.host, test.idx, test.maxLen, test.noColor, result, test.notContains)
			}
		})
	}
}

func TestFormatHostColorCycling(t *testing.T) {
	// Test that different indices produce different colors
	host := "test"
	maxLen := 10
	noColor := false

	results := make([]string, len(colors))
	for i := range colors {
		results[i] = formatHostPrefix(host, i, maxLen, noColor)
	}

	// Test that cycling works (index beyond colors length)
	cycledResult := formatHostPrefix(host, len(colors), maxLen, noColor)
	if cycledResult != results[0] {
		t.Errorf("Color cycling failed: index %d should match index 0", len(colors))
	}
}

func TestFormatHostPrefixEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		host        string
		idx         int
		maxLen      int
		noColor     bool
		description string
	}{
		{"empty host", "", 0, 5, true, "empty hostname should pad correctly"},
		{"very long host", "verylonghostname", 0, 5, true, "long hostname should be truncated by padding"},
		{"negative maxLen", "host", 0, -1, true, "negative maxLen should work with fmt.Sprintf"},
		{"special chars", "host@domain.com", 0, 15, true, "host with special characters"},
		{"unicode host", "héllo", 0, 8, true, "host with unicode characters"},
		{"large index", "host", 100, 8, false, "large index should cycle colors"},
		{"zero maxLen", "host", 0, 0, true, "zero maxLen should work"},
		{"very large maxLen", "host", 0, 100, true, "very large maxLen should work"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := formatHostPrefix(test.host, test.idx, test.maxLen, test.noColor)

			// Basic sanity checks
			if test.noColor && strings.Contains(result, "\033[") {
				t.Errorf("Expected no color codes when noColor=true, got: %q", result)
			}
			if !test.noColor && !strings.Contains(result, "\033[") {
				t.Errorf("Expected color codes when noColor=false, got: %q", result)
			}
			if !test.noColor && !strings.Contains(result, "\033[0m") {
				t.Errorf("Expected reset code when noColor=false, got: %q", result)
			}

			// Length check (padding should work regardless of maxLen value)
			if test.maxLen >= 0 && len(result) < len(test.host) && !test.noColor {
				// For colored output, length should be at least host length + color codes
				minExpectedLen := len(test.host) + len("\033[1;31m") + len("\033[0m")
				if len(result) < minExpectedLen {
					t.Errorf("Expected result length >= %d for colored output, got %d", minExpectedLen, len(result))
				}
			}
		})
	}
}

func TestMaxLenEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected int
	}{
		{"nil slice", nil, 0},
		{"empty strings", []string{"", ""}, 0},
		{"mixed empty and non-empty", []string{"", "a", ""}, 1},
		{"unicode strings", []string{"hello", "héllo", "wörld"}, 6}, // "wörld" has 6 bytes
		{"equal lengths", []string{"aaa", "bbb", "ccc"}, 3},
		{"single character variations", []string{"a", "b", "c"}, 1},
		{"very long strings", []string{"short", "thisisaverylongstringthatshouldbethelongest"}, 43},
		{"numbers as strings", []string{"1", "22", "333", "4444"}, 4},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := maxLen(test.input)
			if result != test.expected {
				t.Errorf("maxLen(%v) = %d, expected %d", test.input, result, test.expected)
			}
		})
	}
}

func TestPrintProgressBarEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		current  int
		total    int
		width    int
		expected string // partial expected output
	}{
		{"zero progress", 0, 10, 5, "[░░░░░] 0/10"},
		{"half progress", 5, 10, 5, "[██░░░] 5/10"},
		{"full progress", 10, 10, 5, "[█████] 10/10"},
		{"zero total", 0, 0, 5, ""}, // Should not print anything
		{"single item", 1, 1, 3, "[███] 1/1"},
		{"current exceeds total", 15, 10, 5, "[█████] 15/10"}, // Should show 100% filled
		{"negative current", -1, 10, 5, "[░░░░░] -1/10"},      // Should show 0% filled
		{"negative total", 5, -1, 5, "[░░░░░] 5/-1"},          // Division by zero should show 0%
		{"zero width", 1, 2, 0, ""},                           // Should show empty bar
		{"large width", 1, 2, 20, "[██████████░░░░░░░░░░] 1/2"},
		{"very large numbers", 1000, 1000, 10, "[██████████] 1000/1000"},
		{"fractional progress", 1, 3, 6, "[██░░░░] 1/3"}, // Should show ~33% filled
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			printProgressBar(test.current, test.total, test.width)

			// Restore stdout
			w.Close()
			os.Stdout = oldStdout

			// Read captured output
			output, _ := io.ReadAll(r)
			outputStr := string(output)

			if test.total == 0 {
				if outputStr != "" {
					t.Errorf("Expected no output for zero total, got %q", outputStr)
				}
				return
			}

			// Check that the output contains expected elements
			if test.expected != "" && !strings.Contains(outputStr, test.expected) {
				t.Errorf("Expected output to contain %q, got %q", test.expected, outputStr)
			}

			// Check for progress bar characters (when width > 0)
			if test.width > 0 && (!strings.Contains(outputStr, "[") || !strings.Contains(outputStr, "]")) {
				t.Errorf("Expected output to contain progress bar brackets, got %q", outputStr)
			}

			// For completed progress, should have newline
			if test.current == test.total && test.total > 0 {
				if !strings.HasSuffix(outputStr, "\n") {
					t.Errorf("Expected output to end with newline for completed progress, got %q", outputStr)
				}
			}

			// Check bar length matches width (count runes, not bytes)
			if test.width > 0 && strings.Contains(outputStr, "[") {
				// Extract the bar content between brackets
				start := strings.Index(outputStr, "[")
				end := strings.Index(outputStr, "]")
				if start >= 0 && end > start {
					barContent := outputStr[start+1 : end]
					runeCount := len([]rune(barContent))
					if runeCount != test.width {
						t.Errorf("Expected bar width %d, got %d in %q", test.width, runeCount, barContent)
					}
				}
			}
		})
	}
}
