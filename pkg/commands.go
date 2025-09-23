package pkg

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
)

// executeCommandStreaming runs a command on all hosts using persistent SSH connections with streaming output and context cancellation
func executeCommandStreaming(ctx context.Context, cm *SSHConnectionManager, hosts []string, command string, noColor bool) {
	maxHostLen := maxLen(hosts)
	var wg sync.WaitGroup

	for i, host := range hosts {
		wg.Go(func() {
			cm.runSSHStreaming(ctx, host, command, i, maxHostLen, noColor)
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

	maxHostLen := maxLen(hosts)
	var wg sync.WaitGroup

	for i, host := range hosts {
		wg.Go(func() {
			runSSHStreaming(ctx, host, command, user, i, maxHostLen, noColor)
		})
	}

	wg.Wait()
}

// uploadFile uploads a file to all hosts in parallel
func uploadFile(hosts []string, filepath, user string, noColor bool) {
	// Check if local file exists
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		fmt.Printf("❌ Error: File '%s' does not exist\n", filepath)
		return
	}

	maxHostLen := maxLen(hosts)
	var wg sync.WaitGroup

	for i, host := range hosts {
		wg.Go(func() {
			runSCP(host, filepath, user, i, maxHostLen, noColor)
		})
	}

	wg.Wait()
}

// runSCP uploads a file to a single host using scp
func runSCP(host, filepath, user string, idx, maxHostLen int, noColor bool) {
	args := []string{"-o", "ConnectTimeout=5", "-o", "BatchMode=yes"}

	if user != "" {
		args = append(args, "-o", "User="+user)
	}

	// Get just the filename for the destination
	filename := filepath
	if strings.Contains(filepath, "/") {
		parts := strings.Split(filepath, "/")
		filename = parts[len(parts)-1]
	}

	// scp source destination
	args = append(args, filepath, host+":"+filename)
	cmd := exec.CommandContext(context.Background(), "scp", args...)

	output, err := cmd.CombinedOutput()
	prefix := formatHostPrefix(host, idx, maxHostLen, noColor)

	if err != nil {
		fmt.Printf("%s: ❌ UPLOAD ERROR: %v\n", prefix, err)
		if len(output) > 0 {
			fmt.Printf("%s: %s\n", prefix, strings.TrimSpace(string(output)))
		}
		return
	}

	fmt.Printf("%s: ✅ Upload successful: %s\n", prefix, filename)
}

// runSSHStreaming executes SSH command for a single host with real-time streaming output
func runSSHStreaming(ctx context.Context, host, command, user string, idx, maxHostLen int, noColor bool) {
	args := []string{"-o", "ConnectTimeout=5", "-o", "BatchMode=yes"}

	if user != "" {
		args = append(args, "-l", user)
	}

	args = append(args, host, command)
	cmd := exec.CommandContext(ctx, "ssh", args...)

	// Get stdout and stderr pipes
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		prefix := formatHostPrefix(host, idx, maxHostLen, noColor)
		fmt.Printf("%s: ERROR: Failed to get stdout pipe: %v\n", prefix, err)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		prefix := formatHostPrefix(host, idx, maxHostLen, noColor)
		fmt.Printf("%s: ERROR: Failed to get stderr pipe: %v\n", prefix, err)
		return
	}

	prefix := formatHostPrefix(host, idx, maxHostLen, noColor)

	// Start goroutines to read and display output in real-time
	var wg sync.WaitGroup

	// Handle stdout
	wg.Go(func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
				line := scanner.Text()
				fmt.Printf("%s: %s\n", prefix, line)
			}
		}
		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			fmt.Printf("%s: ERROR: Failed to read stdout: %v\n", prefix, err)
		}
	})

	// Handle stderr
	wg.Go(func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
				line := scanner.Text()
				fmt.Printf("%s: %s\n", prefix, line)
			}
		}
		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			fmt.Printf("%s: ERROR: Failed to read stderr: %v\n", prefix, err)
		}
	})

	// Start the command
	if err := cmd.Start(); err != nil {
		fmt.Printf("%s: ERROR: Failed to start command: %v\n", prefix, err)
		return
	}

	// Wait for command to complete and output readers to finish
	if err := cmd.Wait(); err != nil {
		// Only show error if context wasn't cancelled
		if ctx.Err() == nil {
			fmt.Printf("%s: ERROR: Command failed: %v\n", prefix, err)
		}
	}
	wg.Wait()
}
