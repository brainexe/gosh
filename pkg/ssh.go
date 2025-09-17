package pkg

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// runCmdWithSeparateOutput runs a command and returns stdout, stderr, and error separately
func runCmdWithSeparateOutput(cmd *exec.Cmd) (string, string, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// RunSSH executes SSH command for a single host
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
