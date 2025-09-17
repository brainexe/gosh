package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// runSSH executes SSH command for a single host
func runSSH(host, command, user string, idx, maxHostLen int, noColor bool) {
	args := []string{"-o", "ConnectTimeout=5", "-o", "BatchMode=yes"}

	if user != "" {
		args = append(args, "-l", user)
	}

	args = append(args, host, command)
	cmd := exec.CommandContext(context.Background(), "ssh", args...)

	output, err := cmd.CombinedOutput()
	prefix := formatHost(host, idx, maxHostLen, noColor)

	if err != nil {
		fmt.Printf("%s: ERROR: %v\n", prefix, err)
		return
	}

	lines := strings.Split(strings.TrimRight(string(output), "\n"), "\n")
	for _, line := range lines {
		if line != "" {
			fmt.Printf("%s: %s\n", prefix, line)
		}
	}
}

// runSCP uploads a file to a single host using scp
func runSCP(host, filepath, user string, idx, maxHostLen int, noColor bool) {
	args := []string{"-o", "ConnectTimeout=5", "-o", "BatchMode=yes"}

	if user != "" {
		args = append(args, "-o", "User="+user)
	}

	// Get just the filename for the destination
	filename := filepath
	if strings.Contains(filepath, "/") {
		parts := strings.Split(filepath, "/")
		filename = parts[len(parts)-1]
	}

	// scp source destination
	args = append(args, filepath, host+":"+filename)
	cmd := exec.CommandContext(context.Background(), "scp", args...)

	output, err := cmd.CombinedOutput()
	prefix := formatHost(host, idx, maxHostLen, noColor)

	if err != nil {
		fmt.Printf("%s: UPLOAD ERROR: %v\n", prefix, err)
		if len(output) > 0 {
			fmt.Printf("%s: %s\n", prefix, strings.TrimSpace(string(output)))
		}
		return
	}

	fmt.Printf("%s: Upload successful: %s\n", prefix, filename)
}
