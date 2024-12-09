package pkg

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

// ExecuteCommandOnHosts executes the given command on all hosts in parallel
func ExecuteCommandOnHosts(hosts []string, command string, userFlag string, noColor bool, sshCmd string) error {
	var wg sync.WaitGroup
	totalHosts := len(hosts)

	for idx, host := range hosts {
		wg.Add(1)
		go func(host string, idx int) {
			defer wg.Done()

			// Add user@host
			userAtHost := host
			if userFlag != "" {
				userAtHost = fmt.Sprintf("%s@%s", userFlag, host)
			}

			// Add the command, wrapped with 'bash -c' to ensure proper execution
			remoteCommand := fmt.Sprintf("bash -c %q", command)

			sshArgs := []string{
				"-tt",                  // Force pseudo-terminal allocation
				"-o", "LogLevel=QUIET", // Suppress warnings
				userAtHost,    // user@host or just host
				remoteCommand, // actual command
			}

			// Prepare the SSH command
			cmd := exec.Command(sshCmd, sshArgs...)

			// Verbose logging
			logrus.Debugf("SSH command: %s %s", sshCmd, strings.Join(sshArgs, " "))

			// Prepare color codes
			colorCode := ""
			if !noColor {
				colorCode = GetColorCode(idx)
			}

			// Create an io.Pipe to capture combined stdout and stderr
			r, w := io.Pipe()
			cmd.Stdout = w
			cmd.Stderr = w

			// Start the command
			if err := cmd.Start(); err != nil {
				logrus.Errorf("Failed to start SSH command for host %s: %v", host, err)
				w.Close()
				return
			}

			// Scan and print the output, line by line
			scanner := bufio.NewScanner(r)
			go func() {
				for scanner.Scan() {
					line := scanner.Text()
					fmt.Printf("%s%s%s: %s\n", colorCode, host, reset, line)
				}
				if err := scanner.Err(); err != nil {
					logrus.Errorf("Error reading output for host %s: %v", host, err)
				}
			}()

			// Wait for the command to finish
			if err := cmd.Wait(); err != nil {
				fmt.Printf("%s%s%s: %v\n", colorCode, host, reset, err)
			}
			w.Close()
		}(host, idx)
	}

	wg.Wait()
	logrus.Debugf("Command executed on %d hosts.", totalHosts)
	return nil
}
