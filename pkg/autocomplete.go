package pkg

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
)

const maxCompletions = 10

// limitCompletions limits the number of completions to a maximum of 10
func limitCompletions(completions []string) []string {
	if len(completions) > maxCompletions {
		return completions[:maxCompletions]
	}
	return completions
}

// customCompleter implements readline.AutoCompleter interface
type customCompleter struct {
	hosts   []string
	noColor bool
	connMgr *SSHConnectionManager
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

	// Get completions using our logic - pass both line and current word
	completions := completerWithWord(lineStr, currentWord, c.hosts, c.connMgr)
	completions = limitCompletions(completions)

	// Convert completions back to rune slices
	result := make([][]rune, len(completions))
	for i, completion := range completions {
		result[i] = []rune(completion)
	}

	return result, wordStart
}

// getLocalFileCompletions gets file completions from current directory (used in :upload only)
func getLocalFileCompletions(prefix string) []string {
	cmd := exec.CommandContext(context.Background(), "bash", "-c", "compgen -f "+prefix) // #nosec G204 -- prefix originates from user typing for local path completion; risk limited to glob expansion. Only used for tab-completion, not executed.
	output, err := cmd.Output()
	if err != nil {
		return []string{}
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var completions []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && line != "." && line != ".." {
			// Check if it's a directory and add trailing slash
			if strings.HasSuffix(line, "/") {
				// Already has trailing slash
				completions = append(completions, line)
			} else {
				// Check if it's a directory by running test -d
				testCmd := exec.CommandContext(context.Background(), "bash", "-c", "test -d '"+line+"' && echo 'dir' || echo 'file'") // #nosec G204 -- line values come from compgen output (local filesystem entries), used only to test directory existence.
				testOutput, testErr := testCmd.Output()
				if testErr == nil && strings.TrimSpace(string(testOutput)) == "dir" {
					completions = append(completions, line+"/")
				} else {
					completions = append(completions, line)
				}
			}
		}
	}
	return limitCompletions(completions)
}

// getSSHCompletions runs completion command on the first host using socket connection
func getSSHCompletions(word string, firstHost string, connMgr *SSHConnectionManager) []string {
	if firstHost == "" {
		return []string{}
	}

	socketPath := connMgr.getSocketPath(firstHost)
	var args []string
	args = append(args, "-S", socketPath, "-o", "BatchMode=yes")
	args = append(args, firstHost)

	// Build compgen command to run on remote host
	var compgenCmd string
	switch {
	case word == "":
		compgenCmd = "compgen -c"
	case strings.Contains(word, "/"):
		// Handle path completion
		compgenCmd = "compgen -d '" + word + "' || compgen -f '" + word + "'"
	default:
		// Try command completion first, then file completion
		compgenCmd = "compgen -c '" + word + "' || compgen -f '" + word + "'"
	}

	args = append(args, compgenCmd)

	cmd := exec.CommandContext(context.Background(), "ssh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return []string{}
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return []string{}
	}

	lines := strings.Split(output, "\n")
	var completions []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && line != "." && line != ".." {
			completions = append(completions, line)
		}
	}

	return completions
}

// completerWithWord handles tab completion with proper word-based logic
func completerWithWord(line string, currentWord string, hosts []string, connMgr *SSHConnectionManager) []string {
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
				// Get all matching files
				cmd := exec.CommandContext(context.Background(), "bash", "-c", "compgen -f "+prefix) // #nosec G204 -- prefix fragment from interactive user input for path suggestions only.
				output, err := cmd.Output()
				if err != nil {
					return []string{}
				}

				lines := strings.Split(strings.TrimSpace(string(output)), "\n")
				var completions []string
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line != "" && line != "." && line != ".." {
						// For internal commands, return the suffix that should be appended
						if strings.HasPrefix(line, prefix) {
							suffix := line[len(prefix):]
							if suffix != "" {
								completions = append(completions, suffix)
							}
						}
					}
				}
				return completions
			}
			// Empty prefix - return all files as full names
			cmd := exec.CommandContext(context.Background(), "bash", "-c", "compgen -f")
			output, err := cmd.Output()
			if err != nil {
				return []string{}
			}
			return strings.Split(strings.TrimSpace(string(output)), "\n")
		}

		// Complete internal commands - return suffixes
		commands := []string{":upload", ":exit", ":help", ":hosts", ":verbose"}
		var matches []string
		for _, cmd := range commands {
			if strings.HasPrefix(cmd, currentWord) {
				suffix := cmd[len(currentWord):]
				if suffix != "" {
					matches = append(matches, suffix+" ")
				}
			}
		}
		return matches
	}

	// For regular commands, use SSH completion on the first host
	sshCompletions := getSSHCompletions(currentWord, hosts[0], connMgr)

	// For all completions (commands and paths), return suffixes as expected by readline
	var filteredCompletions []string
	for _, completion := range sshCompletions {
		if strings.HasPrefix(completion, currentWord) {
			suffix := completion[len(currentWord):]
			if suffix != "" {
				filteredCompletions = append(filteredCompletions, suffix)
			}
		}
	}

	return filteredCompletions
}
