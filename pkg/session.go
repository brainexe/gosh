package pkg

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"os/exec"
	"strings"
	"time"
)

// HostSession represents a session with a remote host using system ssh
type HostSession struct {
	Host         string
	Cmd          *exec.Cmd
	Stdin        io.WriteCloser
	Stdout       io.Reader
	Stderr       io.Reader
	ColorCode    string
	PromptMarker string
}

// NewHostSession creates a new HostSession using the system ssh command
func NewHostSession(host string, userFlag string, idx int, noColor bool, verbose bool, sshCmd string) (*HostSession, error) {
	var username string

	if userFlag != "" {
		username = userFlag + "@"
	} else {
		username = ""
	}

	// Construct SSH arguments
	sshArgs := []string{}

	// Include the '-tt' option to force pseudo-terminal allocation
	sshArgs = append(sshArgs, "-tt")

	// Include verbosity flags if verbose mode is enabled
	if verbose {
		sshArgs = append(sshArgs, "-vvv")
	} else {
		sshArgs = append(sshArgs, "-o", "LogLevel=QUIET")
	}

	// Prepare the prompt marker
	promptMarker := "__POLYSH_PROMPT__"

	// Prepare the remote command
	remoteCommand := fmt.Sprintf(`env PS1="%s" bash --noprofile --norc -i`, promptMarker)

	// Add user@host and remote command
	userAtHost := username + host
	sshArgs = append(sshArgs, userAtHost, remoteCommand)

	// Create the command
	cmd := exec.Command("ssh", sshArgs...)

	// If verbose, print the SSH command being executed
	if verbose {
		logrus.Debugf("SSH command: ssh %s", strings.Join(sshArgs, " "))
	}

	// Create pipes for stdin, stdout, stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdin for host %s: %v", host, err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout for host %s: %v", host, err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stderr for host %s: %v", host, err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start ssh session for host %s: %v", host, err)
	}

	// Add a log message when the connection is established
	if verbose {
		logrus.Infof("Connection established with %s", host)
	}

	colorCode := ""
	if !noColor {
		colorCode = GetColorCode(idx)
	}

	hs := &HostSession{
		Host:         host,
		Cmd:          cmd,
		Stdin:        stdin,
		Stdout:       stdout,
		Stderr:       stderr,
		ColorCode:    colorCode,
		PromptMarker: promptMarker,
	}

	return hs, nil
}

// Close closes the host session
func (hs *HostSession) Close() error {
	// Send exit command to the remote shell
	_, err := hs.Stdin.Write([]byte("exit\n"))
	if err != nil {
		logrus.Errorf("Failed to send exit command to host %s: %v", hs.Host, err)
	}

	// Wait for a short duration to allow the remote shell to exit
	time.Sleep(1 * time.Second)

	// Check if the process is still running
	if hs.Cmd.ProcessState == nil || !hs.Cmd.ProcessState.Exited() {
		// Kill the process
		if err := hs.Cmd.Process.Kill(); err != nil {
			logrus.Errorf("Failed to kill ssh process for host %s: %v", hs.Host, err)
			return err
		}
	}

	// Wait for the command to finish
	if err := hs.Cmd.Wait(); err != nil {
		return fmt.Errorf("failed to wait for ssh session to end for host %s: %v", hs.Host, err)
	}

	return nil
}
