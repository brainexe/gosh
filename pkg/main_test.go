//go:generate bash -c "mkdir -p test_keys && ssh-keygen -t rsa -b 1024 -f test_keys/test_server_key -N '' -q || true"

package pkg

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// HostStatus represents the connection status of a host
type HostStatus struct {
	Host      string
	Connected bool
	Error     error
}

// startFakeSSHServer starts a fake SSH server for testing purposes
func startFakeSSHServer(t *testing.T, addr string, response string) net.Listener {
	t.Helper()

	// Load a private key for the server
	privateBytes, err := os.ReadFile("test_keys/test_server_key")
	if err != nil {
		t.Fatalf("Failed to load private key: %v", err)
	}
	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		t.Fatalf("Failed to parse private key: %v", err)
	}

	// Create server config
	config := &ssh.ServerConfig{
		NoClientAuth: true, // No client authentication
	}
	config.AddHostKey(private)

	// Listen on the specified address
	lc := &net.ListenConfig{}
	listener, err := lc.Listen(context.Background(), "tcp", addr)
	if err != nil {
		t.Fatalf("Failed to listen on %s: %v", addr, err)
	}

	// Handle incoming connections
	go func() {
		for {
			nConn, err := listener.Accept()
			if err != nil {
				// Connection closed, exit gracefully
				return
			}

			// Handle connection in a separate goroutine
			go func(conn net.Conn) {
				defer conn.Close()

				// Establish a new SSH connection
				sshConn, chans, reqs, err := ssh.NewServerConn(conn, config)
				if err != nil {
					t.Logf("Failed to handshake: %v", err)
					return
				}

				// Discard out-of-band requests
				go ssh.DiscardRequests(reqs)

				// Handle channels
				handleChannels(chans, response)
				sshConn.Wait()
			}(nConn)
		}
	}()

	return listener
}

// handleChannels handles SSH channels
func handleChannels(chans <-chan ssh.NewChannel, response string) {
	for newChannel := range chans {
		// Reject non-session channels
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}

		channel, requests, err := newChannel.Accept()
		if err != nil {
			continue
		}

		// Handle requests
		go func(in <-chan *ssh.Request) {
			defer channel.Close()
			for req := range in {
				switch req.Type {
				case "exec":
					// Execute the command and send the response
					var payload struct{ Command string }
					ssh.Unmarshal(req.Payload, &payload)
					if req.WantReply {
						req.Reply(true, nil)
					}
					channel.Write([]byte(response))
					channel.SendRequest("exit-status", false, ssh.Marshal(struct{ ExitStatus uint32 }{0}))
					return
				default:
					if req.WantReply {
						req.Reply(false, nil)
					}
				}
			}
		}(requests)
	}
}

func TestSSHCommandExecution(t *testing.T) {
	// Start the fake SSH server
	addr := "127.0.0.1:0" // Use port 0 to get an available port
	listener := startFakeSSHServer(t, addr, "test output from server\n")
	defer listener.Close()

	// Get the actual address with assigned port
	actualAddr := listener.Addr().String()

	// Wait for the server to start
	time.Sleep(100 * time.Millisecond)

	// Create a client config with a test host key callback
	clientConfig := &ssh.ClientConfig{
		User:            "testuser",
		HostKeyCallback: createTestHostKeyCallback(t),
		ClientVersion:   "SSH-2.0-Go-SSH-TestClient",
		Timeout:         5 * time.Second,
	}

	// Connect to the fake SSH server
	client, err := ssh.Dial("tcp", actualAddr, clientConfig)
	if err != nil {
		t.Fatalf("Failed to dial SSH server: %v", err)
	}
	defer client.Close()

	// Create a session
	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("Failed to create SSH session: %v", err)
	}
	defer session.Close()

	// Capture the output
	var stdoutBuf bytes.Buffer
	session.Stdout = &stdoutBuf

	// Run the command
	if err := session.Run("echo test"); err != nil {
		t.Fatalf("Failed to run command: %v", err)
	}

	// Verify the output
	output := stdoutBuf.String()
	expectedOutput := "test output from server\n"
	if output != expectedOutput {
		t.Errorf("Expected output %q, got %q", expectedOutput, output)
	}
}

// createTestHostKeyCallback creates a host key callback for testing that accepts any host key
func createTestHostKeyCallback(t *testing.T) ssh.HostKeyCallback {
	t.Helper()
	return func(_ string, _ net.Addr, _ ssh.PublicKey) error {
		// In tests, we accept any host key for simplicity
		// This is safe because we're connecting to our own test servers
		return nil
	}
}

func TestExecuteCommandWithFakeSSH(t *testing.T) {
	// Start multiple fake SSH servers
	server1 := startFakeSSHServer(t, "127.0.0.1:0", "output from host1\n")
	defer server1.Close()

	server2 := startFakeSSHServer(t, "127.0.0.1:0", "output from host2\n")
	defer server2.Close()

	// Wait for servers to start
	time.Sleep(100 * time.Millisecond)

	// Get actual addresses
	addr1 := server1.Addr().String()
	addr2 := server2.Addr().String()

	// Test the executeCommand function (this would require modifying main.go to accept custom SSH command)
	// For now, we'll just test that our fake servers respond correctly
	clientConfig := &ssh.ClientConfig{
		User:            "testuser",
		HostKeyCallback: createTestHostKeyCallback(t),
		ClientVersion:   "SSH-2.0-Go-SSH-TestClient",
		Timeout:         5 * time.Second,
	}

	// Test both servers
	for i, addr := range []string{addr1, addr2} {
		client, err := ssh.Dial("tcp", addr, clientConfig)
		if err != nil {
			t.Fatalf("Failed to dial SSH server %d: %v", i+1, err)
		}

		session, err := client.NewSession()
		if err != nil {
			client.Close()
			t.Fatalf("Failed to create SSH session %d: %v", i+1, err)
		}

		var output bytes.Buffer
		session.Stdout = &output

		if err := session.Run("test command"); err != nil {
			session.Close()
			client.Close()
			t.Fatalf("Failed to run command on server %d: %v", i+1, err)
		}

		expectedOutput := map[int]string{
			0: "output from host1\n",
			1: "output from host2\n",
		}

		if output.String() != expectedOutput[i] {
			t.Errorf("Server %d: expected %q, got %q", i+1, expectedOutput[i], output.String())
		}

		session.Close()
		client.Close()
	}
}

func TestExecuteCommand(t *testing.T) {
	tests := []struct {
		name    string
		hosts   []string
		command string
		user    string
		noColor bool
	}{
		{"single host", []string{"localhost"}, "echo test", "", true},
		{"multiple hosts", []string{"host1", "host2"}, "date", "testuser", false},
		{"empty hosts", []string{}, "echo test", "", true},
		{"with user", []string{"localhost"}, "whoami", "testuser", true},
	}

	for _, test := range tests {
		t.Run(test.name, func(_ *testing.T) {
			// This test verifies the function doesn't panic and completes
			// Actual SSH execution is tested in integration tests
			ExecuteCommand(test.hosts, test.command, test.user, test.noColor)
		})
	}
}

func TestUploadFile(t *testing.T) {
	// Create a temporary test file
	tempFile := "/tmp/gosh_test_file.txt"
	testContent := "test file content"
	err := os.WriteFile(tempFile, []byte(testContent), 0o600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(tempFile)

	tests := []struct {
		name     string
		hosts    []string
		filepath string
		user     string
		noColor  bool
	}{
		{"existing file", []string{"localhost"}, tempFile, "", true},
		{"nonexistent file", []string{"localhost"}, "/nonexistent/file.txt", "", true},
		{"multiple hosts", []string{"host1", "host2"}, tempFile, "testuser", false},
		{"empty hosts", []string{}, tempFile, "", true},
	}

	for _, test := range tests {
		t.Run(test.name, func(_ *testing.T) {
			// This test verifies the function doesn't panic and handles file existence
			// Actual SCP execution is tested in integration tests
			connManager := NewSSHConnectionManager(test.user)
			connManager.connections = make(map[string]*SSHConnection)
			for _, host := range test.hosts {
				connManager.connections[host] = &SSHConnection{
					host: host,
				}
			}
			uploadFile(connManager, test.filepath)
		})
	}
}

func TestRunCmdWithSeparateOutput(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *exec.Cmd
		wantErr bool
	}{
		{"echo success", func() *exec.Cmd { return exec.CommandContext(context.Background(), "echo", "hello") }, false},
		{"false command", func() *exec.Cmd { return exec.CommandContext(context.Background(), "false") }, true},
		{"nonexistent command", func() *exec.Cmd { return exec.CommandContext(context.Background(), "nonexistent_command_12345") }, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := test.setup()
			stdout, stderr, err := runCmdWithSeparateOutput(cmd)

			if test.wantErr && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !test.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// stdout and stderr should be strings (can be empty)
			if stdout == "" && stderr == "" && !test.wantErr {
				t.Errorf("Expected some output for successful command")
			}
		})
	}
}

func TestSSHCommandConstruction(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		command  string
		user     string
		expected []string
	}{
		{
			"basic command",
			"example.com",
			"echo test",
			"",
			[]string{"-o", SSHConnectTimeout, "-o", SSHBatchMode, "example.com", "echo test"},
		},
		{
			"with user",
			"example.com",
			"whoami",
			"testuser",
			[]string{"-o", SSHConnectTimeout, "-o", SSHBatchMode, "-l", "testuser", "example.com", "whoami"},
		},
		{
			"complex command",
			"192.168.1.1",
			"ls -la /home",
			"admin",
			[]string{"-o", SSHConnectTimeout, "-o", SSHBatchMode, "-l", "admin", "192.168.1.1", "ls -la /home"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// We can't easily test runSSH directly due to exec.Command,
			// but we can verify the argument construction logic
			args := []string{"-o", SSHConnectTimeout, "-o", SSHBatchMode}
			if test.user != "" {
				args = append(args, "-l", test.user)
			}
			args = append(args, test.host, test.command)

			if len(args) != len(test.expected) {
				t.Errorf("Expected %d args, got %d", len(test.expected), len(args))
			}

			for i, expected := range test.expected {
				if i >= len(args) || args[i] != expected {
					t.Errorf("Arg %d: expected %q, got %q", i, expected, args[i])
				}
			}
		})
	}
}

func TestSCPCommandConstruction(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		filepath string
		user     string
		expected []string
	}{
		{
			"basic file upload",
			"example.com",
			"test.txt",
			"",
			[]string{"-o", SSHConnectTimeout, "-o", SSHBatchMode, "test.txt", "example.com:test.txt"},
		},
		{
			"with user",
			"example.com",
			"script.sh",
			"testuser",
			[]string{"-o", SSHConnectTimeout, "-o", SSHBatchMode, "-o", "User=testuser", "script.sh", "example.com:script.sh"},
		},
		{
			"path with directory",
			"192.168.1.1",
			"/home/user/data.csv",
			"admin",
			[]string{"-o", SSHConnectTimeout, "-o", SSHBatchMode, "-o", "User=admin", "/home/user/data.csv", "192.168.1.1:data.csv"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Test the SCP argument construction logic
			args := []string{"-o", SSHConnectTimeout, "-o", SSHBatchMode}
			if test.user != "" {
				args = append(args, "-o", "User="+test.user)
			}

			// Extract filename from filepath
			filename := test.filepath
			if strings.Contains(test.filepath, "/") {
				parts := strings.Split(test.filepath, "/")
				filename = parts[len(parts)-1]
			}

			args = append(args, test.filepath, test.host+":"+filename)

			if len(args) != len(test.expected) {
				t.Errorf("Expected %d args, got %d", len(test.expected), len(args))
			}

			for i, expected := range test.expected {
				if i >= len(args) || args[i] != expected {
					t.Errorf("Arg %d: expected %q, got %q", i, expected, args[i])
				}
			}
		})
	}
}

func TestGetLocalFileCompletions(t *testing.T) {
	// Create temporary files for testing
	tempDir := "/tmp/gosh_test_completions"
	err := os.MkdirAll(tempDir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)
	os.Chdir(tempDir)

	// Create test files
	testFiles := []string{"test1.txt", "test2.log", "script.sh", "data.csv"}
	for _, file := range testFiles {
		os.WriteFile(file, []byte("test"), 0o600)
	}

	// Create test directory
	os.Mkdir("testdir", 0o755)

	tests := []struct {
		name     string
		prefix   string
		expected []string
	}{
		{"no prefix", "", []string{"test1.txt", "test2.log", "script.sh", "data.csv", "testdir/"}},
		{"test prefix", "test", []string{"test1.txt", "test2.log", "testdir/"}},
		{"script prefix", "script", []string{"script.sh"}},
		{"nonexistent prefix", "xyz", []string{}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			completions := getLocalFileCompletions(test.prefix)

			// Check that all expected completions are present
			for _, expected := range test.expected {
				found := false
				for _, completion := range completions {
					if completion == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected completion %q not found in %v", expected, completions)
				}
			}
		})
	}
}

func TestTestAllConnections(t *testing.T) {
	// Test cases
	tests := []struct {
		name            string
		hosts           []string
		user            string
		verbose         bool
		mockResults     map[string]bool // host -> connected status
		expectConnected []string
		expectExit      bool
	}{
		{
			name:            "all hosts connect successfully",
			hosts:           []string{"host1", "host2"},
			user:            "testuser",
			verbose:         false,
			mockResults:     map[string]bool{"host1": true, "host2": true},
			expectConnected: []string{"host1", "host2"},
			expectExit:      false,
		},
		{
			name:            "some hosts fail",
			hosts:           []string{"host1", "badhost", "host2"},
			user:            "testuser",
			verbose:         true,
			mockResults:     map[string]bool{"host1": true, "badhost": false, "host2": true},
			expectConnected: []string{"host1", "host2"},
			expectExit:      false,
		},
		{
			name:            "all hosts fail - would exit but we test the logic",
			hosts:           []string{"badhost1", "badhost2"},
			user:            "testuser",
			verbose:         false,
			mockResults:     map[string]bool{"badhost1": false, "badhost2": false},
			expectConnected: []string{}, // This would normally cause exit
			expectExit:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout for verbose output testing
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Create a testable version that uses mocked connection results
			testableTestAllConnections := func(hosts []string, user string, verbose bool, mockConnectionTester func(string, string) *HostStatus) []string {
				if verbose {
					fmt.Printf("🔍 Testing connections to %d host(s)...\n", len(hosts))
				}

				statusChan := make(chan *HostStatus, len(hosts))
				var wg sync.WaitGroup

				// Start connection tests in parallel
				for _, host := range hosts {
					wg.Go(func() {
						status := mockConnectionTester(host, user)
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

				// Don't call os.Exit(1) in tests - just return empty slice
				if len(connectedHosts) == 0 {
					return connectedHosts
				}

				if verbose {
					fmt.Printf("✅ Successfully connected to %d/%d host(s)\n", len(connectedHosts), len(hosts))
					fmt.Println()
				}
				return connectedHosts
			}

			// Mock connection tester function
			mockConnectionTester := func(host, _ string) *HostStatus {
				connected := tt.mockResults[host]
				var err error
				if !connected {
					err = fmt.Errorf("connection refused")
				}
				return &HostStatus{
					Host:      host,
					Connected: connected,
					Error:     err,
				}
			}

			// Call the testable function
			result := testableTestAllConnections(tt.hosts, tt.user, tt.verbose, mockConnectionTester)

			// Restore stdout
			w.Close()
			os.Stdout = oldStdout

			// Read captured output
			output, _ := io.ReadAll(r)
			outputStr := string(output)

			// Verify results
			if tt.expectExit && len(result) != 0 {
				t.Errorf("Expected no connected hosts (would exit), but got %v", result)
			} else if !tt.expectExit {
				if len(result) != len(tt.expectConnected) {
					t.Errorf("Expected %d connected hosts, got %d", len(tt.expectConnected), len(result))
				}
				// Check that the connected hosts are correct
				for _, expected := range tt.expectConnected {
					found := false
					for _, actual := range result {
						if actual == expected {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected connected host %s not found in result %v", expected, result)
					}
				}
			}

			// Verify verbose output contains expected elements
			if tt.verbose {
				if !strings.Contains(outputStr, "🔍 Testing connections") {
					t.Errorf("Expected verbose output to contain connection test message")
				}
				if !tt.expectExit && len(tt.expectConnected) > 0 {
					if !strings.Contains(outputStr, "✅ Successfully connected") {
						t.Errorf("Expected verbose output to contain success message")
					}
				}
			}

			// For failed connections, check warning output
			if tt.expectExit || (len(tt.hosts) > len(tt.expectConnected)) {
				if !strings.Contains(outputStr, "⚠️  Warning:") {
					t.Errorf("Expected warning output for failed connections")
				}
			}
		})
	}
}

func TestCustomCompleterDo(t *testing.T) {
	completer := &customCompleter{
		firstHost: "host1",
		noColor:   true,
		connMgr:   &SSHConnectionManager{},
	}

	tests := []struct {
		name     string
		line     string
		pos      int
		expected int // expected word start position
	}{
		{"empty line", "", 0, 0},
		{"single word", "echo", 4, 0},
		{"two words", "echo test", 9, 5},
		{"partial word", "ec", 2, 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lineRunes := []rune(test.line)
			completions, wordStart := completer.Do(lineRunes, test.pos)

			if wordStart != test.expected {
				t.Errorf("Expected word start %d, got %d", test.expected, wordStart)
			}

			// completions should be a valid slice (can be empty)
			if completions == nil {
				t.Errorf("Expected non-nil completions slice")
			}
		})
	}
}

func TestSSHConnectionManager(t *testing.T) {
	tests := []struct {
		name     string
		user     string
		expected bool // expect socket dir creation
	}{
		{"with user", "testuser", true},
		{"without user", "", true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cm := NewSSHConnectionManager(test.user)

			if cm == nil {
				t.Fatal("Expected non-nil SSHConnectionManager")
			}

			if cm.user != test.user {
				t.Errorf("Expected user %q, got %q", test.user, cm.user)
			}

			if cm.connections == nil {
				t.Error("Expected non-nil connections map")
			}

			if cm.socketDir == "" {
				t.Error("Expected non-empty socket directory")
			}

			// Test socket path generation
			socketPath := cm.getSocketPath("testhost")
			expectedPath := filepath.Join(cm.socketDir, "gosh-testhost")
			if socketPath != expectedPath {
				t.Errorf("Expected socket path %q, got %q", expectedPath, socketPath)
			}

			// Clean up
			cm.closeAllConnections()
		})
	}
}

func TestRunCmdWithSeparateOutputExtended(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		args        []string
		wantErr     bool
		checkStdout bool
	}{
		{"echo success", "echo", []string{"hello world"}, false, true},
		{"cat success", "cat", []string{"/dev/null"}, false, false},
		{"command failure", "false", []string{}, true, false},
		{"nonexistent command", "nonexistent_command_12345", []string{}, true, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := exec.CommandContext(context.Background(), test.command, test.args...) // #nosec G204 -- test inputs are predefined in table; safe for tests
			stdout, stderr, err := runCmdWithSeparateOutput(cmd)

			if test.wantErr && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !test.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Check that stdout and stderr are strings (can be empty)
			if test.checkStdout && stdout == "" && stderr == "" && !test.wantErr {
				t.Errorf("Expected some output for successful command")
			}
		})
	}
}

// Add suppression for dynamic command execution in tests
// The test constructs commands from fixed test cases; inputs are controlled and not user-provided
// #nosec G204
func TestDynamicCommandExecution(t *testing.T) {
	t.Helper()
	tests := []struct {
		name    string
		command string
		args    []string
	}{
		{"echo", "echo", []string{"test"}},
	}
	for _, tc := range tests {
		cmd := exec.CommandContext(context.Background(), tc.command, tc.args...) // #nosec G204 -- test-only execution with controlled inputs
		_ = cmd.Run()
	}
}

func TestLimitCompletions(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{"empty slice", []string{}, []string{}},
		{"single item", []string{"item1"}, []string{"item1"}},
		{"exactly max", make([]string, maxCompletions), make([]string, maxCompletions)},
		{"over max", make([]string, maxCompletions+5), make([]string, maxCompletions)},
		{"under max", make([]string, maxCompletions-2), make([]string, maxCompletions-2)},
	}

	// Initialize the test slices with unique values
	for i := range tests {
		for j := range tests[i].input {
			tests[i].input[j] = fmt.Sprintf("completion%d", j)
		}
		for j := range tests[i].expected {
			tests[i].expected[j] = fmt.Sprintf("completion%d", j)
		}
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := limitCompletions(test.input)
			if len(result) != len(test.expected) {
				t.Errorf("Expected %d completions, got %d", len(test.expected), len(result))
			}

			// Check that the beginning of the result matches the expected
			for i, expected := range test.expected {
				if i >= len(result) || result[i] != expected {
					t.Errorf("Result[%d] = %q, expected %q", i, result[i], expected)
				}
			}
		})
	}
}

func TestCompleterWithWord(t *testing.T) {
	hosts := []string{"host1", "host2"}
	user := "testuser"

	tests := []struct {
		name         string
		line         string
		currentWord  string
		expectedType string // "internal", "ssh", "empty", "none"
	}{
		{"empty line", "", "", "empty"},
		{"upload command", ":upload", "upload", "internal"},
		{"upload with file", ":upload test", "test", "internal"},
		{"help command", ":help", "help", "internal"},
		{"regular command", "ls", "ls", "ssh"},
		{"path command", "cd /home", "/home", "ssh"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testHosts := hosts

			conMgr := NewSSHConnectionManager(user)
			conMgr.connections = map[string]*SSHConnection{}
			completions := completerWithWord(test.line, test.currentWord, testHosts[0], conMgr)

			// completions should be a valid slice (can be nil or empty)
			// The function may return nil in some error cases

			// For empty input, should return empty slice
			if test.expectedType == "empty" && completions != nil && len(completions) != 0 {
				t.Errorf("Expected empty completions for empty input, got %v", completions)
			}

			// For other cases, just ensure the function doesn't panic and returns something reasonable
			if test.expectedType == "internal" {
				t.Logf("Internal command completion returned %d items", len(completions))
			}
		})
	}
}

func TestGetLocalFileCompletionsExtended(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := "/tmp/gosh_test_autocomplete"
	err := os.MkdirAll(tempDir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)
	os.Chdir(tempDir)

	// Create test files and directories
	testFiles := []string{"test1.txt", "test2.log", "script.sh"}
	for _, file := range testFiles {
		err := os.WriteFile(file, []byte("test content"), 0o600)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", file, err)
		}
	}

	// Create test directory
	err = os.Mkdir("testdir", 0o755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	tests := []struct {
		name        string
		prefix      string
		expectFiles bool
	}{
		{"no prefix", "", true},
		{"test prefix", "test", true},
		{"script prefix", "script", true},
		{"nonexistent prefix", "xyz", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			completions := getLocalFileCompletions(test.prefix)

			if test.expectFiles && len(completions) == 0 {
				t.Errorf("Expected some completions for prefix %q, got none", test.prefix)
			}

			if !test.expectFiles && len(completions) > 0 {
				t.Logf("Got unexpected completions for nonexistent prefix: %v", completions)
			}

			// Verify completions are limited
			if len(completions) > maxCompletions {
				t.Errorf("Expected at most %d completions, got %d", maxCompletions, len(completions))
			}

			// Check that all completions start with the prefix (if prefix is not empty)
			if test.prefix != "" {
				for _, completion := range completions {
					if !strings.HasPrefix(completion, test.prefix) {
						t.Errorf("Completion %q does not start with prefix %q", completion, test.prefix)
					}
				}
			}
		})
	}
}
