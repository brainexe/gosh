package pkg

import (
	"bytes"
	"net"
	"os"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// Start a fake SSH server for testing purposes
func startFakeSSHServer(t *testing.T, addr string, response string) (net.Listener, error) {
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
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to listen on %s: %v", addr, err)
	}

	// Handle incoming connections
	go func() {
		for {
			nConn, err := listener.Accept()
			if err != nil {
				t.Logf("Failed to accept incoming connection: %v", err)
				continue
			}

			// Establish a new SSH connection
			sshConn, chans, reqs, err := ssh.NewServerConn(nConn, config)
			if err != nil {
				t.Logf("Failed to handshake: %v", err)
				continue
			}

			// Discard out-of-band requests
			go ssh.DiscardRequests(reqs)

			// Handle channels
			go handleChannels(chans, response)
			sshConn.Wait()
		}
	}()

	return listener, nil
}

// Handle SSH channels
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
					channel.Close()
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
	addr := "127.0.0.1:2222"
	listener, err := startFakeSSHServer(t, addr, "5\n")
	if err != nil {
		t.Fatalf("Failed to start fake SSH server: %v", err)
	}
	defer listener.Close()

	// Wait for the server to start
	time.Sleep(1 * time.Second)

	// Create a client config
	clientConfig := &ssh.ClientConfig{
		User:            "testuser",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		ClientVersion:   "SSH-2.0-Go-SSH-TestClient",
	}

	// Connect to the fake SSH server
	client, err := ssh.Dial("tcp", addr, clientConfig)
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
	if err := session.Run("echo 5"); err != nil {
		t.Fatalf("Failed to run command: %v", err)
	}

	// Verify the output
	output := stdoutBuf.String()
	expectedOutput := "5\n"
	if output != expectedOutput {
		t.Errorf("Expected output %q, got %q", expectedOutput, output)
	}
}
