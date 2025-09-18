package pkg

import (
	"fmt"
	"strings"
	"sync"

	"github.com/chzyer/readline"
)

// printProgressBar displays a simple text-based progress bar
func printProgressBar(current, total int, width int) {
	if total == 0 {
		return
	}

	percentage := float64(current) / float64(total)
	filled := int(float64(width) * percentage)

	bar := ""
	for i := range width {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}

	fmt.Printf("\r[%s] %d/%d (%.1f%%)", bar, current, total, percentage*100)
	if current == total {
		fmt.Println() // New line when complete
	}
}

// InteractiveMode starts an interactive session
func InteractiveMode(hosts []string, user string, noColor bool, verbose bool) {
	if verbose {
		fmt.Printf("🔍 Testing connections to %d host(s)...\n", len(hosts))
		fmt.Println("💡 Type 'exit' or 'quit' to exit, 'help' for help")
	}

	// Create SSH connection manager for persistent connections
	connManager := NewSSHConnectionManager(user)
	defer connManager.CleanupAllConnections() // Ensure cleanup on exit

	// Establish connections to all hosts in parallel with progress bar
	type connectionResult struct {
		host  string
		error error
	}

	resultChan := make(chan connectionResult, len(hosts))
	var wg sync.WaitGroup

	// Start connections in parallel
	for _, host := range hosts {
		wg.Go(func() {
			err := connManager.establishConnection(host)
			resultChan <- connectionResult{host: host, error: err}
		})
	}

	// Close channel when all connections complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Process results and update progress
	var connectedHosts []string
	var failedConnections []string
	completed := 0

	for result := range resultChan {
		completed++
		if result.error != nil {
			failedConnections = append(failedConnections, fmt.Sprintf("%s: %v", result.host, result.error))
		} else {
			connectedHosts = append(connectedHosts, result.host)
		}

		// Show progress bar
		printProgressBar(completed, len(hosts), 20)
	}

	// Show any connection failures
	if len(failedConnections) > 0 {
		fmt.Printf("⚠️  Failed to establish persistent connections to %d host(s):\n", len(failedConnections))
		for _, failure := range failedConnections {
			fmt.Printf("  • %s\n", failure)
		}
		fmt.Println()
	}

	// Check if we have any working connections
	if len(connectedHosts) == 0 {
		fmt.Println("❌ Error: No hosts are reachable. Exiting.")
		return
	}

	if verbose {
		fmt.Printf("🚀 Interactive mode - connected to %d/%d host(s)\n", len(connectedHosts), len(hosts))
	}

	// Create readline instance
	prompt := fmt.Sprintf("🖥️ [%d]> ", len(connectedHosts))
	config := &readline.Config{
		Prompt: prompt,
	}

	rl, err := readline.NewEx(config)
	if err != nil {
		fmt.Printf("⚠️  Failed to initialize readline: %v\n", err)
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
		case line == "help" || line == ":help":
			ShowHelp()
		case strings.HasPrefix(line, ":upload "):
			filepath := strings.TrimSpace(strings.TrimPrefix(line, ":upload"))
			if filepath == "" {
				fmt.Println("📁 Usage: :upload <filepath>")
				continue
			}
			UploadFile(connectedHosts, filepath, user, noColor)
		default:
			// Use persistent connections for better performance
			ExecuteCommandPersistent(connManager, connectedHosts, line, noColor)
		}
	}
}

// ShowHelp displays help information
func ShowHelp() {
	fmt.Println("📚 Commands:")
	fmt.Println("  :help            - Show this help")
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
