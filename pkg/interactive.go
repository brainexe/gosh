package pkg

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"

	"github.com/chzyer/readline"
)

// Verbose Global verbose flag that can be toggled in interactive mode
var Verbose bool

// InteractiveMode starts an interactive session
func InteractiveMode(hosts []string, user string, noColor bool, verbose bool) {
	// Set the global verbose flag to support changes during the session
	Verbose = verbose

	if Verbose {
		fmt.Printf("🔍 Testing connections to %d host(s)...\n", len(hosts))
		fmt.Println("💡 Type 'exit' or 'quit' to exit, 'help' for help")
	}

	// Create SSH connection manager for persistent connections
	connManager := NewSSHConnectionManager(user)
	defer connManager.closeAllConnections() // Ensure cleanup on exit

	if Verbose {
		fmt.Printf("Socket directory: %s\n", connManager.socketDir)
	}
	// Establish connections to all hosts in parallel with progress bar
	type connectionResult struct {
		host  string
		error error
	}

	resultChan := make(chan connectionResult, len(hosts))
	var wg sync.WaitGroup

	// Start connections in parallel
	for idx, host := range hosts {
		wg.Go(func() {
			err, _ := connManager.establishConnection(host, idx, maxLen(hosts), noColor)
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

	if Verbose {
		fmt.Printf("🚀 Interactive mode - connected to %d/%d host(s)\n", len(connectedHosts), len(hosts))
	}

	// Create readline instance
	prompt := fmt.Sprintf("🖥️ [%d]> ", len(connectedHosts))
	config := &readline.Config{
		Prompt: prompt,
		// todo tweak
		AutoComplete: &customCompleter{
			firstHost: connectedHosts[0],
			noColor:   noColor,
			connMgr:   connManager,
		},
		HistoryFile: os.Getenv("HOME") + "/.gosh_history",
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
			return
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		switch {
		case line == ":exit" || line == ":quit":
			return
		case line == ":help":
			showHelp()
		case line == ":hosts":
			fmt.Printf("🖥️ Connected hosts (%d):\n", len(connectedHosts))
			for _, host := range connectedHosts {
				fmt.Printf("  • %s\n", host)
			}
		case strings.HasPrefix(line, ":upload "):
			filepath := strings.TrimSpace(strings.TrimPrefix(line, ":upload"))
			if filepath == "" {
				fmt.Println("📁 Usage: :upload <filepath>")
				continue
			}
			uploadFile(connManager, filepath)
		case line == ":verbose":
			Verbose = !Verbose
			status := "disabled"
			if Verbose {
				status = "enabled"
			}
			fmt.Printf("🔍 Verbose mode %s\n", status)
		default:
			// All commands use streaming output - simple and real-time!
			// Create a cancellable context for interrupt handling
			ctx, cancel := context.WithCancel(context.Background())

			// Set up signal handling for Ctrl+C
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt)

			// Goroutine to handle interrupt signal
			go func() {
				<-sigChan
				fmt.Println("\n🛑 Command interrupted by user")
				cancel()
			}()

			// Execute command with interruptible context
			executeCommandStreaming(ctx, connManager, line)

			// Clean up
			cancel()
			signal.Stop(sigChan)
		}
	}
}

// showHelp displays help information
func showHelp() {
	fmt.Println("📚 Commands:")
	fmt.Println("  :help            - Show this help")
	fmt.Println("  :upload <file>   - Upload file to all hosts (current directory)")
	fmt.Println("  :exit/:quit      - Exit interactive mode")
	fmt.Println("  :hosts       	- List connected hosts")
	fmt.Println("  :verbose         - Toggle verbose output mode")
	fmt.Println("  <command>        - Execute command on all connected hosts")
	fmt.Println()
	fmt.Println("💡 Examples:")
	fmt.Println("  date            - Show date/time on all connected hosts")
	fmt.Println("  uptime          - Show uptime on all connected hosts")
	fmt.Println("  ls -la          - List files on all connected hosts")
	fmt.Println("  :upload script.sh - Upload script.sh to all connected hosts")
}
