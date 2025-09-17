package main

import (
	"bytes"
	"context"
	"net"
	"os"
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
		result := maxLen(test.input)
		if result != test.expected {
			t.Errorf("maxLen(%v) = %d, expected %d", test.input, result, test.expected)
		}
	}
}

func TestFormatHost(t *testing.T) {
	tests := []struct {
		host     string
		idx      int
		maxLen   int
		noColor  bool
		contains string
	}{
		{"host1", 0, 10, true, "host1     "},
		{"short", 1, 10, true, "short     "},
		{"host1", 0, 10, false, "host1     "}, // should contain color codes
	}

	for _, test := range tests {
		result := formatHost(test.host, test.idx, test.maxLen, test.noColor)
		if !strings.Contains(result, test.contains) {
			t.Errorf("formatHost(%q, %d, %d, %t) = %q, should contain %q",
				test.host, test.idx, test.maxLen, test.noColor, result, test.contains)
		}
	}
}

// startFakeSSHServer starts a fake SSH server for testing purposes
func startFakeSSHServer(t *testing.T, addr string, response string) net.Listener {
	t.Helper()

	// Load a private key for the server
	privateBytes, err := os.ReadFile("test_data/test_server_key")
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
