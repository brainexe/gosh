package pkg

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// runCmdWithSeparateOutput runs a command and returns stdout, stderr, and error separately
func runCmdWithSeparateOutput(cmd *exec.Cmd) (string, string, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// SSHConnectionManager manages persistent SSH connections using ControlMaster
type SSHConnectionManager struct {
	mu          sync.Mutex
	connections map[string]*SSHConnection
	socketDir   string
	user        string
}

// SSHConnection represents a persistent SSH connection to a host
type SSHConnection struct {
	host       string
	socketPath string
	connected  bool
}

// NewSSHConnectionManager creates a new connection manager
func NewSSHConnectionManager(user string) *SSHConnectionManager {
	socketDir := filepath.Join(os.TempDir(), "gosh-ssh-sockets")
	if err := os.MkdirAll(socketDir, 0o700); err != nil {
		// If we can't create the directory, we'll handle it when establishing connections
		socketDir = os.TempDir() // fallback to temp dir
	}

	return &SSHConnectionManager{
		connections: make(map[string]*SSHConnection),
		socketDir:   socketDir,
		user:        user,
	}
}

// getSocketPath returns the socket path for a host
func (cm *SSHConnectionManager) getSocketPath(host string) string {
	return filepath.Join(cm.socketDir, "gosh-"+strings.ReplaceAll(host, "/", "_"))
}

// establishConnection establishes a persistent SSH connection to a host
func (cm *SSHConnectionManager) establishConnection(host string) error {
	socketPath := cm.getSocketPath(host)

	// Establish new connection
	args := []string{
		"-M",             // Enable ControlMaster
		"-S", socketPath, // Control socket path
		"-o", "ControlPersist=10m", // Keep connection alive for 10 minutes
		"-o", "ConnectTimeout=5",
		"-o", "BatchMode=yes",
		"-f", // Go to background after establishing connection
	}

	if cm.user != "" {
		args = append(args, "-l", cm.user)
	}

	args = append(args, host, "true") // Simple command to establish connection

	cmd := exec.CommandContext(context.Background(), "ssh", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to establish SSH connection to %s: %w", host, err)
	}

	// Store connection info
	cm.mu.Lock()
	cm.connections[host] = &SSHConnection{
		host:       host,
		socketPath: socketPath,
		connected:  true,
	}
	cm.mu.Unlock()

	return nil
}

// runSSHPersistentStreaming executes SSH command using persistent connection with real-time streaming output and context cancellation
func (cm *SSHConnectionManager) runSSHPersistentStreaming(ctx context.Context, host, command string, idx, maxHostLen int, noColor bool) {
	socketPath := cm.getSocketPath(host)

	args := []string{
		"-S", socketPath, // Use existing control socket
		"-o", "BatchMode=yes",
	}

	if cm.user != "" {
		args = append(args, "-l", cm.user)
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

// cleanupConnection closes a persistent SSH connection
func (cm *SSHConnectionManager) cleanupConnection(host string) {
	if conn, exists := cm.connections[host]; exists {
		// Close the SSH control connection
		// Note: We need to be careful with the host parameter, but since it's controlled by our code
		// and stored in our connections map, it should be safe
		// #nosec G204 - host parameter is controlled by our connection manager, not user input
		cmd := exec.CommandContext(context.Background(), "ssh", "-S", conn.socketPath, "-O", "exit", host)
		_ = cmd.Run() // Ignore errors, connection might already be closed

		// Remove socket file
		_ = os.Remove(conn.socketPath)

		delete(cm.connections, host)
	}
}

// cleanupAllConnections closes all persistent SSH connections
func (cm *SSHConnectionManager) cleanupAllConnections() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for host := range cm.connections {
		cm.cleanupConnection(host)
	}

	// Remove socket directory
	_ = os.RemoveAll(cm.socketDir)
}
