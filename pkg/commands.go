package pkg

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"time"
)

// executeCommandStreaming runs a command on all hosts using persistent SSH connections with streaming output and context cancellation
func executeCommandStreaming(ctx context.Context, cm *SSHConnectionManager, command string) {
	var wg sync.WaitGroup

	for _, connection := range cm.connections {
		wg.Go(func() {
			cm.runSSHStreaming(ctx, connection, command)
		})
	}

	wg.Wait()
}

// ExecuteCommand runs a command on all hosts with streaming output and interrupt handling (no persistent connections)
func ExecuteCommand(hosts []string, command, user string, noColor bool) {
	// Create a cancellable context for interrupt handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	defer signal.Stop(sigChan)

	// Goroutine to handle interrupt signal
	go func() {
		<-sigChan
		fmt.Println("\n🛑 Command interrupted by user")
		cancel()
	}()

	// Create SSH connection manager for persistent connections
	connManager := NewSSHConnectionManager(user)
	defer connManager.closeAllConnections() // Ensure cleanup on exit

	maxHostLen := maxLen(hosts)
	var wg sync.WaitGroup
	for i, host := range hosts {
		wg.Go(func() {
			err, conn := connManager.establishConnection(host, i, maxHostLen, noColor)
			if err != nil {
				fmt.Printf("Failed to establish connection to %s: %v\n", host, err)
				return
			}

			connManager.runSSHStreaming(ctx, conn, command)
		})
	}

	wg.Wait()
}

// uploadFile uploads a file to all hosts in parallel
func uploadFile(connManager *SSHConnectionManager, filepath string) {
	// Check if local file exists
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		fmt.Printf("❌ Error: File '%s' does not exist\n", filepath)
		return
	}

	var wg sync.WaitGroup
	for _, conn := range connManager.connections {
		wg.Go(func() {
			runSCP(connManager, conn, filepath)
		})
	}

	wg.Wait()
}

// runSCP uploads a file to a single host using scp with direct connection and progress
func runSCP(cm *SSHConnectionManager, conn *SSHConnection, filepath string) {
	args := []string{
		"-o", SSHConnectTimeout,
		"-o", SSHBatchMode,
		"-o", "ControlPath=" + conn.socketPath, // Use the control socket for persistent connection
		"-v", // Verbose mode for progress output
	}

	if cm.user != "" {
		args = append(args, "-o", "User="+cm.user)
	}

	// Get just the filename for the destination
	filename := filepath
	if strings.Contains(filepath, "/") {
		parts := strings.Split(filepath, "/")
		filename = parts[len(parts)-1]
	}

	start := time.Now()

	// scp source destination
	args = append(args, filepath, conn.host+":"+filename)
	cmd := exec.CommandContext(context.Background(), "scp", args...)

	if Verbose {
		fmt.Printf("SCP connand: %s\n", cmd.String())
	}

	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	if err != nil {
		fmt.Printf("%s: ❌ UPLOAD ERROR (%.2fs): %v\n", conn.prefix, duration.Seconds(), err)
		if len(output) > 0 {
			fmt.Printf("%s: %s\n", conn.prefix, strings.TrimSpace(string(output)))
		}
		return
	}

	fmt.Printf("%s: ✅ Upload successful: %s (%.1fs)\n", conn.prefix, filename, duration.Seconds())
}
