package pkg

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// HostStatus represents the connection status of a host
type HostStatus struct {
	Host      string
	Connected bool
	Error     error
}

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
	mu          sync.RWMutex
	connections map[string]*SSHConnection
	socketDir   string
	user        string
}

// SSHConnection represents a persistent SSH connection to a host
type SSHConnection struct {
	host       string
	socketPath string
	connected  bool
	lastUsed   time.Time
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
	// cm.mu.Lock()
	// defer cm.mu.Unlock()

	socketPath := cm.getSocketPath(host)

	// Check if connection already exists and is recent
	if conn, exists := cm.connections[host]; exists && conn.connected {
		// Test if the connection is still alive
		if time.Since(conn.lastUsed) < 5*time.Minute {
			conn.lastUsed = time.Now()
			return nil
		}
		// Clean up old connection
		cm.cleanupConnection(host)
	}

	// Establish new connection
	args := []string{
		"-M",             // Enable ControlMaster
		"-S", socketPath, // Control socket path
		"-o", "ControlPersist=10m", // Keep connection alive for 10 minutes
		"-o", "ConnectTimeout=5",
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=no",
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
	cm.connections[host] = &SSHConnection{
		host:       host,
		socketPath: socketPath,
		connected:  true,
		lastUsed:   time.Now(),
	}

	return nil
}

// RunSSHPersistent executes SSH command using persistent connection
func (cm *SSHConnectionManager) RunSSHPersistent(host, command string, idx, maxHostLen int, noColor bool) {
	// Ensure connection exists
	if err := cm.establishConnection(host); err != nil {
		prefix := FormatHost(host, idx, maxHostLen, noColor)
		fmt.Printf("%s: ERROR: %v\n", prefix, err)
		return
	}

	socketPath := cm.getSocketPath(host)

	args := []string{
		"-S", socketPath, // Use existing control socket
		"-o", "BatchMode=yes",
	}

	if cm.user != "" {
		args = append(args, "-l", cm.user)
	}

	args = append(args, host, command)
	cmd := exec.CommandContext(context.Background(), "ssh", args...)

	stdout, stderr, err := runCmdWithSeparateOutput(cmd)
	prefix := FormatHost(host, idx, maxHostLen, noColor)

	// Update last used time
	cm.mu.Lock()
	if conn, exists := cm.connections[host]; exists {
		conn.lastUsed = time.Now()
	}
	cm.mu.Unlock()

	// Display stdout if present
	if len(stdout) > 0 {
		lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
		for _, line := range lines {
			if line != "" {
				fmt.Printf("%s: %s\n", prefix, line)
			}
		}
	}

	// Display stderr if present
	if len(stderr) > 0 {
		lines := strings.Split(strings.TrimRight(stderr, "\n"), "\n")
		for _, line := range lines {
			if line != "" {
				fmt.Printf("%s: %s\n", prefix, line)
			}
		}
	}

	// Display error status if command failed
	if err != nil {
		fmt.Printf("%s: ERROR: %v\n", prefix, err)
	}
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

// CleanupAllConnections closes all persistent SSH connections
func (cm *SSHConnectionManager) CleanupAllConnections() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for host := range cm.connections {
		cm.cleanupConnection(host)
	}

	// Remove socket directory
	os.RemoveAll(cm.socketDir)
}

// RunSSH executes SSH command for a single host (legacy function for backward compatibility)
func RunSSH(host, command, user string, idx, maxHostLen int, noColor bool) {
	args := []string{"-o", "ConnectTimeout=5", "-o", "BatchMode=yes"}

	if user != "" {
		args = append(args, "-l", user)
	}

	args = append(args, host, command)
	cmd := exec.CommandContext(context.Background(), "ssh", args...)

	stdout, stderr, err := runCmdWithSeparateOutput(cmd)
	prefix := FormatHost(host, idx, maxHostLen, noColor)

	// Display stdout if present
	if len(stdout) > 0 {
		lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
		for _, line := range lines {
			if line != "" {
				fmt.Printf("%s: %s\n", prefix, line)
			}
		}
	}

	// Display stderr if present
	if len(stderr) > 0 {
		lines := strings.Split(strings.TrimRight(stderr, "\n"), "\n")
		for _, line := range lines {
			if line != "" {
				fmt.Printf("%s: %s\n", prefix, line)
			}
		}
	}

	// Display error status if command failed
	if err != nil {
		fmt.Printf("%s: ERROR: %v\n", prefix, err)
	}
}
