package pkg

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/chzyer/readline"
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
	words := strings.Fields(lineStr)
	var currentWord string
	var wordStart int

	if len(words) > 0 && !strings.HasSuffix(lineStr, " ") {
		// We're completing the last word
		currentWord = words[len(words)-1]
		wordStart = pos - len([]rune(currentWord))
	} else {
		// We're starting a new word
		currentWord = ""
		wordStart = pos
	}

	// Get completions using our existing logic
	completions := Completer(lineStr, c.hosts, c.user)

	// Filter completions that start with the current word
	var filtered []string
	for _, completion := range completions {
		if strings.HasPrefix(completion, currentWord) {
			// Only show the part that needs to be added
			filtered = append(filtered, completion[len(currentWord):])
		}
	}

	// Convert completions back to rune slices
	result := make([][]rune, len(filtered))
	for i, completion := range filtered {
		result[i] = []rune(completion)
	}

	// Return completions and the position where they should be inserted
	return result, wordStart + len([]rune(currentWord))
}

// getSSHCompletions gets completion suggestions from the first host using SSH
func getSSHCompletions(line string, hosts []string, user string) []string {
	if len(hosts) == 0 {
		return []string{}
	}

	// Use the first host for completion
	host := hosts[0]

	// Prepare SSH command to get completions
	// We'll use bash's programmable completion if available
	args := []string{"-o", "ConnectTimeout=5", "-o", "BatchMode=yes"}
	if user != "" {
		args = append(args, "-l", user)
	}

	// Try to get completions using compgen (bash built-in)
	compCommand := fmt.Sprintf("compgen -c '%s' 2>/dev/null || compgen -f '%s' 2>/dev/null || echo ''", line, line)
	args = append(args, host, compCommand)

	cmd := exec.CommandContext(context.Background(), "ssh", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return []string{}
	}

	// Parse the output and return as suggestions
	completions := []string{}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, completion := range lines {
		completion = strings.TrimSpace(completion)
		if completion != "" && completion != line {
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
	line = strings.TrimSpace(line)
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

// InteractiveMode starts an interactive session
func InteractiveMode(hosts []string, user string, noColor bool) {
	fmt.Println("Interactive mode - type commands to execute on all hosts")
	fmt.Println("Type 'exit' or 'quit' to exit, 'help' for help")

	// Create custom completer that integrates with readline
	completer := &customCompleter{
		hosts:   hosts,
		user:    user,
		noColor: noColor,
	}

	// Create readline instance with completer
	config := &readline.Config{
		Prompt:       "gosh> ",
		AutoComplete: completer,
	}

	rl, err := readline.NewEx(config)
	if err != nil {
		fmt.Printf("Failed to initialize readline: %v\n", err)
		// Fallback to basic input
		fallbackInteractiveMode(hosts, user, noColor)
		return
	}
	defer rl.Close()

	for {
		line, err := rl.Readline()
		if err != nil { // EOF or Ctrl+D
			break
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		switch {
		case line == "exit" || line == "quit":
			return
		case line == "help":
			ShowHelp()
		case strings.HasPrefix(line, ":upload "):
			filepath := strings.TrimSpace(strings.TrimPrefix(line, ":upload"))
			if filepath == "" {
				fmt.Println("Usage: :upload <filepath>")
				continue
			}
			UploadFile(hosts, filepath, user, noColor)
		default:
			ExecuteCommand(hosts, line, user, noColor)
		}
	}
}

// fallbackInteractiveMode provides basic input when readline fails
func fallbackInteractiveMode(hosts []string, user string, noColor bool) {
	fmt.Println("Using fallback input mode (no tab completion)")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("gosh> ")
		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		switch {
		case line == "exit" || line == "quit":
			return
		case line == "help":
			ShowHelp()
		case strings.HasPrefix(line, ":upload "):
			filepath := strings.TrimSpace(strings.TrimPrefix(line, ":upload"))
			if filepath == "" {
				fmt.Println("Usage: :upload <filepath>")
				continue
			}
			UploadFile(hosts, filepath, user, noColor)
		default:
			ExecuteCommand(hosts, line, user, noColor)
		}
	}
}

// ShowHelp displays help information
func ShowHelp() {
	fmt.Println("Commands:")
	fmt.Println("  help       - Show this help")
	fmt.Println("  exit/quit  - Exit interactive mode")
	fmt.Println("  :upload <file>    - Upload file to all hosts (current directory)")
	fmt.Println("  <command>  - Execute command on all hosts")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  date       - Show date/time on all hosts")
	fmt.Println("  uptime     - Show uptime on all hosts")
	fmt.Println("  ls -la     - List files on all hosts")
	fmt.Println("  :upload script.sh - Upload script.sh to all hosts")
}
