package pkg

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

const (
	// SSHConnectTimeout is the SSH connection timeout in seconds
	SSHConnectTimeout = "ConnectTimeout=5"
	// SSHControlPersist is how long SSH control connections should persist
	SSHControlPersist = "ControlPersist=10m"
	// SSHBatchMode enables non-interactive operation
	SSHBatchMode = "BatchMode=yes"
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
	prefix     string
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
func (cm *SSHConnectionManager) establishConnection(host string, idx int, maxLen int, noColor bool) (*SSHConnection, error) {
	socketPath := cm.getSocketPath(host)

	// Establish new connection
	args := []string{
		"-M",             // Enable ControlMaster
		"-S", socketPath, // Control socket path
		"-o", SSHControlPersist, // Keep connection alive for 10 minutes
		"-o", SSHConnectTimeout,
		"-o", SSHBatchMode,
		"-f", // Go to background after establishing connection
	}

	if cm.user != "" {
		args = append(args, "-l", cm.user)
	}

	args = append(args, host, "true") // Simple command to establish connection

	cmd := exec.CommandContext(context.Background(), "ssh", args...)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to establish SSH connection to %s: %w", host, err)
	}

	conn := &SSHConnection{
		host:       host,
		socketPath: socketPath,
		prefix:     formatHostPrefix(host, idx, maxLen, noColor),
	}
	// Store connection info, only for successful hosts
	cm.mu.Lock()
	cm.connections[host] = conn
	cm.mu.Unlock()

	return conn, nil
}

// runSSHStreaming executes SSH command using persistent connection with real-time streaming output and context cancellation
func (cm *SSHConnectionManager) runSSHStreaming(ctx context.Context, conn *SSHConnection, command string) {
	args := []string{
		"-S", conn.socketPath, // Use existing control socket
		"-o", SSHBatchMode,
	}

	if cm.user != "" {
		args = append(args, "-l", cm.user)
	}

	args = append(args, conn.host, command)
	cmd := exec.CommandContext(ctx, "ssh", args...)

	// Get stdout and stderr pipes
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Printf("%s: ERROR: Failed to get stdout pipe: %v\n", conn.prefix, err)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Printf("%s: ERROR: Failed to get stderr pipe: %v\n", conn.prefix, err)
		return
	}

	// Helper function to handle output streams
	handleStream := func(reader io.Reader, output *os.File) func() {
		return func() {
			scanner := bufio.NewScanner(reader)
			for scanner.Scan() {
				select {
				case <-ctx.Done():
					return
				default:
					line := scanner.Text()
					_, _ = fmt.Fprintf(output, "%s: %s\n", conn.prefix, line)
				}
			}
			if err := scanner.Err(); err != nil && ctx.Err() == nil {
				_, _ = fmt.Fprintf(os.Stderr, "%s: ERROR: Failed to read: %v\n", conn.prefix, err)
			}
		}
	}

	// Start goroutines to read and display output in real-time
	var wg sync.WaitGroup
	wg.Go(handleStream(stdout, os.Stdout))
	wg.Go(handleStream(stderr, os.Stderr))

	// Start the command
	if err := cmd.Start(); err != nil {
		fmt.Printf("%s: ERROR: Failed to start command: %v\n", conn.prefix, err)
		return
	}

	// Wait for command to complete and output readers to finish
	if err := cmd.Wait(); err != nil {
		// Only show error if context wasn't cancelled
		if ctx.Err() == nil {
			fmt.Printf("%s: ERROR: Command failed: %v\n", conn.prefix, err)
		}
	}
	wg.Wait()
}

// closeConnection closes a persistent SSH connection
func (cm *SSHConnectionManager) closeConnection(host string) {
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

// closeAllConnections closes all persistent SSH connections
func (cm *SSHConnectionManager) closeAllConnections() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for host := range cm.connections {
		cm.closeConnection(host)
	}
}
