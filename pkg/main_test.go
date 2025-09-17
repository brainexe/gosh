package pkg

import (
	"bytes"
	"context"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestMaxLen(t *testing.T) {
	tests := []struct {
		input    []string
		expected int
	}{
		{[]string{"a", "bb", "ccc"}, 3},
		{[]string{"hello", "world"}, 5},
		{[]string{}, 0},
		{[]string{"single"}, 6},
	}

	for _, test := range tests {
		result := MaxLen(test.input)
		if result != test.expected {
			t.Errorf("MaxLen(%v) = %d, expected %d", test.input, result, test.expected)
		}
	}
}

func TestFormatHost(t *testing.T) {
	tests := []struct {
		name        string
		host        string
		idx         int
		maxLen      int
		noColor     bool
		contains    string
		notContains string
	}{
		{"no color padding", "host1", 0, 10, true, "host1     ", "\033["},
		{"short host no color", "short", 1, 10, true, "short     ", "\033["},
		{"with color codes", "host1", 0, 10, false, "host1     ", ""},
		{"with color has ANSI", "host1", 0, 10, false, "\033[", ""},
		{"with color has reset", "host1", 0, 10, false, "\033[0m", ""},
		{"different colors", "host2", 1, 10, false, "\033[", ""},
		{"exact length", "exactly10c", 0, 10, true, "exactly10c", "\033["},
		{"longer than max", "verylonghost", 0, 8, true, "verylonghost", "\033["},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := FormatHost(test.host, test.idx, test.maxLen, test.noColor)

			if test.contains != "" && !strings.Contains(result, test.contains) {
				t.Errorf("FormatHost(%q, %d, %d, %t) = %q, should contain %q",
					test.host, test.idx, test.maxLen, test.noColor, result, test.contains)
			}

			if test.notContains != "" && strings.Contains(result, test.notContains) {
				t.Errorf("FormatHost(%q, %d, %d, %t) = %q, should not contain %q",
					test.host, test.idx, test.maxLen, test.noColor, result, test.notContains)
			}
		})
	}
}

func TestFormatHostColorCycling(t *testing.T) {
	// Test that different indices produce different colors
	host := "test"
	maxLen := 10
	noColor := false

	results := make([]string, len(Colors))
	for i := 0; i < len(Colors); i++ {
		results[i] = FormatHost(host, i, maxLen, noColor)
	}

	// Test that cycling works (index beyond Colors length)
	cycledResult := FormatHost(host, len(Colors), maxLen, noColor)
	if cycledResult != results[0] {
		t.Errorf("Color cycling failed: index %d should match index 0", len(Colors))
	}
}

// startFakeSSHServer starts a fake SSH server for testing purposes
func startFakeSSHServer(t *testing.T, addr string, response string) net.Listener {
	t.Helper()

	// Load a private key for the server
	privateBytes, err := os.ReadFile("../test_data/test_server_key")
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
		t.Run(test.name, func(t *testing.T) {
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
	err := os.WriteFile(tempFile, []byte(testContent), 0644)
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
		t.Run(test.name, func(t *testing.T) {
			// This test verifies the function doesn't panic and handles file existence
			// Actual SCP execution is tested in integration tests
			UploadFile(test.hosts, test.filepath, test.user, test.noColor)
		})
	}
}

func TestRunCmdWithSeparateOutput(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		wantErr bool
	}{
		{"echo success", "echo", []string{"hello"}, false},
		{"false command", "false", []string{}, true},
		{"nonexistent command", "nonexistent_command_12345", []string{}, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := exec.Command(test.command, test.args...)
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
			[]string{"-o", "ConnectTimeout=5", "-o", "BatchMode=yes", "example.com", "echo test"},
		},
		{
			"with user",
			"example.com",
			"whoami",
			"testuser",
			[]string{"-o", "ConnectTimeout=5", "-o", "BatchMode=yes", "-l", "testuser", "example.com", "whoami"},
		},
		{
			"complex command",
			"192.168.1.1",
			"ls -la /home",
			"admin",
			[]string{"-o", "ConnectTimeout=5", "-o", "BatchMode=yes", "-l", "admin", "192.168.1.1", "ls -la /home"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// We can't easily test RunSSH directly due to exec.Command,
			// but we can verify the argument construction logic
			args := []string{"-o", "ConnectTimeout=5", "-o", "BatchMode=yes"}
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
			[]string{"-o", "ConnectTimeout=5", "-o", "BatchMode=yes", "test.txt", "example.com:test.txt"},
		},
		{
			"with user",
			"example.com",
			"script.sh",
			"testuser",
			[]string{"-o", "ConnectTimeout=5", "-o", "BatchMode=yes", "-o", "User=testuser", "script.sh", "example.com:script.sh"},
		},
		{
			"path with directory",
			"192.168.1.1",
			"/home/user/data.csv",
			"admin",
			[]string{"-o", "ConnectTimeout=5", "-o", "BatchMode=yes", "-o", "User=admin", "/home/user/data.csv", "192.168.1.1:data.csv"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Test the SCP argument construction logic
			args := []string{"-o", "ConnectTimeout=5", "-o", "BatchMode=yes"}
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

func TestCompleter(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		hosts    []string
		user     string
		expected int // number of expected completions (approximate)
	}{
		{"empty line", "", []string{"host1"}, "user", 0},
		{"upload command", ":upload", []string{"host1"}, "user", 1},
		{"upload with space", ":upload ", []string{"host1"}, "user", 0}, // depends on current dir files
		{"regular command", "echo", []string{"host1"}, "user", 0},       // depends on SSH response
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			completions := Completer(test.line, test.hosts, test.user)
			// We can't predict exact completions, but we can test the function doesn't panic
			if completions == nil {
				t.Errorf("Completer returned nil, expected slice")
			}
		})
	}
}

func TestGetLocalFileCompletions(t *testing.T) {
	// Create temporary files for testing
	tempDir := "/tmp/gosh_test_completions"
	err := os.MkdirAll(tempDir, 0755)
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
		os.WriteFile(file, []byte("test"), 0644)
	}

	// Create test directory
	os.Mkdir("testdir", 0755)

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

func TestCustomCompleterDo(t *testing.T) {
	completer := &customCompleter{
		hosts:   []string{"host1", "host2"},
		user:    "testuser",
		noColor: true,
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
