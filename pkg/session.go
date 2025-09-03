package pkg

import (
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/sirupsen/logrus"
)

// HostSession represents a session with a remote host using system ssh
type HostSession struct {
	Host   string
	Prefix string
	Cmd    *exec.Cmd
	Stdin  io.WriteCloser
	Stdout io.Reader
	Stderr io.Reader
}

// newHostSession creates a new HostSession using the system ssh command
func newHostSession(host string, userFlag string, idx int, noColor bool, sshCmd string, maxHostLen int) (*HostSession, error) {
	// Prepare the remote command
	// Unset PROMPT_COMMAND, set PS1, and exec bash
	remoteCommand := `sh -i -c 'PS1=""; ENV=; export PS1 ENV; exec sh -i'`

	sshArgs := getSSHArgs(host, userFlag)
	sshArgs = append(sshArgs, remoteCommand)

	cmd := exec.Command(sshCmd, sshArgs...)

	// Create pipes for stdin, stdout, stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdin for host %s: %w", host, err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout for host %s: %w", host, err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stderr for host %s: %w", host, err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start ssh session for host %s: %w", host, err)
	}

	// Add a log message when the connection is established
	logrus.Debugf("Connection established with %s", host)

	hs := &HostSession{
		Host:   host,
		Prefix: getPrefix(host, noColor, idx, maxHostLen),
		Cmd:    cmd,
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	}

	return hs, nil
}

// Close closes the host session
func (hs *HostSession) Close() error {
	// Send exit command to the remote shell
	_, err := hs.Stdin.Write([]byte("exit\n"))
	if err != nil {
		logrus.Warnf("Failed to send exit command to host %s: %v", hs.Host, err)
	}

	// Wait for a short duration to allow the remote shell to exit
	time.Sleep(100 * time.Millisecond)

	// Check if the process is still running
	if err != nil || hs.Cmd.ProcessState == nil || !hs.Cmd.ProcessState.Exited() {
		// Kill the process
		if err := hs.Cmd.Process.Kill(); err != nil {
			logrus.Errorf("Failed to kill ssh process for host %s: %v", hs.Host, err)
			return err
		}
	}

	// Wait for the command to finish
	if err := hs.Cmd.Wait(); err != nil {
		return fmt.Errorf("failed to wait for ssh session to end for host %s: %w", hs.Host, err)
	}

	return nil
}
