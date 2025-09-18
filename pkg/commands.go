package pkg

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// ExecuteCommand runs a command on all hosts in parallel
func ExecuteCommand(hosts []string, command, user string, noColor bool) {
	maxHostLen := MaxLen(hosts)
	var wg sync.WaitGroup

	for i, host := range hosts {
		wg.Go(func() {
			RunSSH(host, command, user, i, maxHostLen, noColor)
		})
	}

	wg.Wait()
}

// ExecuteCommandPersistent runs a command on all hosts using persistent SSH connections
func ExecuteCommandPersistent(cm *SSHConnectionManager, hosts []string, command string, noColor bool) {
	maxHostLen := MaxLen(hosts)
	var wg sync.WaitGroup

	for i, host := range hosts {
		wg.Go(func() {
			cm.RunSSHPersistent(host, command, i, maxHostLen, noColor)
		})
	}

	wg.Wait()
}

// UploadFile uploads a file to all hosts in parallel
func UploadFile(hosts []string, filepath, user string, noColor bool) {
	// Check if local file exists
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		fmt.Printf("❌ Error: File '%s' does not exist\n", filepath)
		return
	}

	maxHostLen := MaxLen(hosts)
	var wg sync.WaitGroup

	for i, host := range hosts {
		wg.Go(func() {
			RunSCP(host, filepath, user, i, maxHostLen, noColor)
		})
	}

	wg.Wait()
}

// RunSCP uploads a file to a single host using scp
func RunSCP(host, filepath, user string, idx, maxHostLen int, noColor bool) {
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
	prefix := FormatHost(host, idx, maxHostLen, noColor)

	if err != nil {
		fmt.Printf("%s: ❌ UPLOAD ERROR: %v\n", prefix, err)
		if len(output) > 0 {
			fmt.Printf("%s: %s\n", prefix, strings.TrimSpace(string(output)))
		}
		return
	}

	fmt.Printf("%s: ✅ Upload successful: %s\n", prefix, filename)
}
