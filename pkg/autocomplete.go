package pkg

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// customCompleter implements readline.AutoCompleter interface
type customCompleter struct {
	hosts   []string
	user    string
	noColor bool
}

// Do implements the AutoCompleter interface
func (c *customCompleter) Do(line []rune, pos int) ([][]rune, int) {
	// Convert current line to string up to cursor position
	lineStr := string(line[:pos])

	// Find the start of the current word being completed
	wordStart := pos
	for i := pos - 1; i >= 0; i-- {
		if line[i] == ' ' || line[i] == '\t' {
			wordStart = i + 1
			break
		}
		if i == 0 {
			wordStart = 0
		}
	}

	// Extract the current word being completed
	currentWord := string(line[wordStart:pos])

	// Get completions using our existing logic
	completions := Completer(lineStr, c.hosts, c.user)

	// Filter and process completions
	var filtered []string
	for _, completion := range completions {
		completion = strings.TrimSpace(completion)
		if completion == "" || completion == "." || completion == ".." {
			continue
		}

		// For file completions, we need to handle the path properly
		// Both path and simple completions use the same suffix extraction logic
		if strings.HasPrefix(completion, currentWord) {
			// Extract only the part that should be appended
			// For currentWord="/v" and completion="/var", we want "ar"
			// For currentWord="l" and completion="ls", we want "s"
			suffix := completion[len(currentWord):]
			filtered = append(filtered, suffix)
		}
	}

	// Convert completions back to rune slices
	result := make([][]rune, len(filtered))
	for i, completion := range filtered {
		result[i] = []rune(completion)
	}

	// Return completions and the position where the word starts
	return result, wordStart
}

// getSSHCompletions gets completion suggestions from the first host using SSH
func getSSHCompletions(line string, hosts []string, user string) []string {
	if len(hosts) == 0 {
		return []string{}
	}

	// Use the first host for completion
	host := hosts[0]

	// Extract the word being completed
	words := strings.Fields(line)
	if len(words) == 0 {
		return []string{}
	}

	// Get the last word for completion
	lastWord := ""
	if !strings.HasSuffix(line, " ") {
		lastWord = words[len(words)-1]
	}

	// Prepare SSH command to get completions
	args := []string{"-o", "ConnectTimeout=2", "-o", "BatchMode=yes"}
	if user != "" {
		args = append(args, "-l", user)
	}

	// Create a more sophisticated completion command
	var compCommand string
	if len(words) == 1 && !strings.HasSuffix(line, " ") {
		// Completing command name - use which and ls /usr/bin as fallback
		compCommand = fmt.Sprintf("(which %s* 2>/dev/null; ls /usr/bin/%s* /bin/%s* 2>/dev/null | head -10) | sort -u | head -20", lastWord, lastWord, lastWord)
	} else {
		// Completing file/directory names - use ls with glob pattern
		dir := "."
		pattern := lastWord
		if strings.Contains(lastWord, "/") {
			parts := strings.Split(lastWord, "/")
			if len(parts) > 1 {
				dir = strings.Join(parts[:len(parts)-1], "/")
				if dir == "" {
					dir = "/"
				}
				pattern = parts[len(parts)-1]
			}
		}

		// Build the correct completion command
		if pattern == "" {
			compCommand = fmt.Sprintf("ls -1a '%s'/ 2>/dev/null | head -20", dir)
		} else {
			// Use find to get files/directories that start with pattern
			if dir == "/" {
				compCommand = fmt.Sprintf("find / -maxdepth 1 -name '%s*' 2>/dev/null | head -20", pattern)
			} else {
				compCommand = fmt.Sprintf("find '%s' -maxdepth 1 -name '%s*' 2>/dev/null | head -20", dir, pattern)
			}
		}
	}

	args = append(args, host, compCommand)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ssh", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return []string{}
	}

	// Parse the output and return as suggestions
	completions := []string{}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, completion := range lines {
		completion = strings.TrimSpace(completion)
		if completion != "" {
			completions = append(completions, completion)
		}
	}

	return completions
}

// getLocalFileCompletions gets file completions from current directory
func getLocalFileCompletions(prefix string) []string {
	var completions []string

	// Get current directory files
	files, err := os.ReadDir(".")
	if err != nil {
		return completions
	}

	for _, file := range files {
		name := file.Name()
		if strings.HasPrefix(name, prefix) {
			if file.IsDir() {
				completions = append(completions, name+"/")
			} else {
				completions = append(completions, name)
			}
		}
	}

	return completions
}

// Completer handles tab completion for the interactive mode
func Completer(line string, hosts []string, user string) []string {
	if line == "" {
		return []string{}
	}

	// Handle internal commands starting with ":"
	if strings.HasPrefix(line, ":") {
		if strings.HasPrefix(line, ":upload ") {
			// Complete filenames for :upload command
			parts := strings.SplitN(line, " ", 2)
			if len(parts) == 2 {
				prefix := parts[1]
				return getLocalFileCompletions(prefix)
			}
			return getLocalFileCompletions("")
		}
		// Complete internal commands
		commands := []string{":upload"}
		var matches []string
		for _, cmd := range commands {
			if strings.HasPrefix(cmd, line) {
				matches = append(matches, cmd+" ")
			}
		}
		return matches
	}

	// For regular commands, get completions from first host
	return getSSHCompletions(line, hosts, user)
}
