package pkg

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/chzyer/readline"
)

// HostStatus represents the connection status of a host
type HostStatus struct {
	Host      string
	Connected bool
	Error     error
}

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

	// Debug logging
	log.Printf("[DEBUG] Tab completion: line='%s', pos=%d, currentWord='%s', wordStart=%d", lineStr, pos, currentWord, wordStart)

	// Get completions using our existing logic
	completions := Completer(lineStr, c.hosts, c.user)
	log.Printf("[DEBUG] Got %d raw completions: %v", len(completions), completions)

	// Filter and process completions
	var filtered []string
	for _, completion := range completions {
		completion = strings.TrimSpace(completion)
		if completion == "" || completion == "." || completion == ".." {
			continue
		}

		// For file completions, we need to handle the path properly
		if strings.Contains(currentWord, "/") {
			// For paths like "/v", we expect completions like "/var", "/vagrant", etc.
			// readline expects only the suffix that completes the current word
			if strings.HasPrefix(completion, currentWord) {
				// Extract only the part that should be appended
				// For currentWord="/v" and completion="/var", we want "ar"
				suffix := completion[len(currentWord):]
				filtered = append(filtered, suffix)
			}
		} else {
			// For simple completions (like command names), extract suffix
			if strings.HasPrefix(completion, currentWord) {
				// Extract only the part that should be appended
				// For currentWord="l" and completion="ls", we want "s"
				suffix := completion[len(currentWord):]
				filtered = append(filtered, suffix)
			}
		}
	}

	// Convert completions back to rune slices
	result := make([][]rune, len(filtered))
	for i, completion := range filtered {
		result[i] = []rune(completion)
	}

	log.Printf("[DEBUG] Returning %d filtered completions: %v, wordStart=%d", len(filtered), filtered, wordStart)
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
	log.Printf("[DEBUG] Using host '%s' for completion (first of %d hosts)", host, len(hosts))

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

		log.Printf("[DEBUG] Parsed path: dir='%s', pattern='%s' from lastWord='%s'", dir, pattern, lastWord)

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

	log.Printf("[DEBUG] SSH completion command: ssh %v", args)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ssh", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		log.Printf("[DEBUG] SSH completion failed: %v", err)
		return []string{}
	}

	//	log.Printf("[DEBUG] SSH completion output: %s", string(output))

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

// testHostConnection tests if a host is reachable via SSH
func testHostConnection(host, user string) *HostStatus {
	args := []string{"-o", "ConnectTimeout=3", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=no"}
	if user != "" {
		args = append(args, "-l", user)
	}
	args = append(args, host, "true") // Simple command that should always succeed

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ssh", args...)
	err := cmd.Run()

	return &HostStatus{
		Host:      host,
		Connected: err == nil,
		Error:     err,
	}
}

// testAllConnections tests connectivity to all hosts and provides real-time updates
func testAllConnections(hosts []string, user string, verbose bool) []string {
	if verbose {
		fmt.Printf("🔍 Testing connections to %d host(s)...\n", len(hosts))
	}

	statusChan := make(chan *HostStatus, len(hosts))
	var wg sync.WaitGroup

	// Start connection tests in parallel
	for _, host := range hosts {
		wg.Go(func() {
			status := testHostConnection(host, user)
			statusChan <- status
		})
	}

	// Close channel when all tests complete
	go func() {
		wg.Wait()
		close(statusChan)
	}()

	var connectedHosts []string
	var failedHosts []string
	completed := 0

	// Process results as they come in
	for status := range statusChan {
		completed++
		if status.Connected {
			connectedHosts = append(connectedHosts, status.Host)
			if verbose {
				fmt.Printf("✓ %s connected [%d/%d]\n", status.Host, completed, len(hosts))
			}
		} else {
			failedHosts = append(failedHosts, status.Host)
			if verbose {
				fmt.Printf("✗ %s failed: %v [%d/%d]\n", status.Host, status.Error, completed, len(hosts))
			}
		}
	}

	// Always show warnings for failed hosts (concise)
	if len(failedHosts) > 0 {
		fmt.Printf("⚠️  Warning: %d host(s) failed to connect:\n", len(failedHosts))
		for _, host := range failedHosts {
			fmt.Printf("  • %s\n", host)
		}
		// Add newline after warnings to separate from prompt
		fmt.Println()
	}

	if len(connectedHosts) == 0 {
		fmt.Println("❌ Error: No hosts are reachable. Exiting.")
		os.Exit(1)
	}

	if verbose {
		fmt.Printf("✅ Successfully connected to %d/%d host(s)\n", len(connectedHosts), len(hosts))
		fmt.Println()
	}
	return connectedHosts
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

// InteractiveMode starts an interactive session
func InteractiveMode(hosts []string, user string, noColor bool, verbose bool) {
	// Test connectivity to all hosts first
	connectedHosts := testAllConnections(hosts, user, verbose)

	if verbose {
		fmt.Printf("\n🚀 Interactive mode - connected to %d host(s)\n", len(connectedHosts))
		fmt.Println("💡 Type 'exit' or 'quit' to exit, 'help' for help")
	}

	// Create custom completer that integrates with readline
	completer := &customCompleter{
		hosts:   connectedHosts,
		user:    user,
		noColor: noColor,
	}

	// Create readline instance with completer
	prompt := fmt.Sprintf("🖥️ [%d]> ", len(connectedHosts))
	config := &readline.Config{
		Prompt:       prompt,
		AutoComplete: completer,
	}

	rl, err := readline.NewEx(config)
	if err != nil {
		fmt.Printf("⚠️  Failed to initialize readline: %v\n", err)
		// Fallback to basic input
		fallbackInteractiveMode(connectedHosts, user, noColor, verbose)
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
				fmt.Println("📁 Usage: :upload <filepath>")
				continue
			}
			UploadFile(connectedHosts, filepath, user, noColor)
		default:
			ExecuteCommand(connectedHosts, line, user, noColor)
		}
	}
}

// fallbackInteractiveMode provides basic input when readline fails
func fallbackInteractiveMode(hosts []string, user string, noColor bool, verbose bool) {
	if verbose {
		fmt.Println("⚠️  Using fallback input mode (no tab completion)")
	}

	prompt := fmt.Sprintf("🖥️ [%d]> ", len(hosts))
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print(prompt)
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
				fmt.Println("📁 Usage: :upload <filepath>")
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
	fmt.Println("📚 Commands:")
	fmt.Println("  help            - Show this help")
	fmt.Println("  exit/quit       - Exit interactive mode")
	fmt.Println("  :upload <file>  - Upload file to all hosts (current directory)")
	fmt.Println("  <command>       - Execute command on all connected hosts")
	fmt.Println()
	fmt.Println("💡 Examples:")
	fmt.Println("  date            - Show date/time on all connected hosts")
	fmt.Println("  uptime          - Show uptime on all connected hosts")
	fmt.Println("  ls -la          - List files on all connected hosts")
	fmt.Println("  :upload script.sh - Upload script.sh to all connected hosts")
}
