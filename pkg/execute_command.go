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
	totalHosts := len(hosts)
	maxHostLen := maxHostName(hosts)

	var wg sync.WaitGroup
	wg.Add(totalHosts)

	for idx, host := range hosts {
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
				colorCode = getColorCode(idx)
			}
			padding := getPadding(host, maxHostLen)

			// Create an io.Pipe to capture combined stdout and stderr
			r, w := io.Pipe()
			cmd.Stdout = w
			cmd.Stderr = w
			defer w.Close()

			// Start the command
			if err := cmd.Start(); err != nil {
				logrus.Errorf("Failed to start SSH command for host %s: %v", host, err)
				return
			}

			// Scan and print the output, line by line
			scanner := bufio.NewScanner(r)
			go func() {
				for scanner.Scan() {
					line := scanner.Text()
					fmt.Printf("%s%s%s%s: %s\n", colorCode, host, ansiReset, padding, line)
				}
				if err := scanner.Err(); err != nil {
					logrus.Errorf("Error reading output for host %s: %v", host, err)
				}
			}()

			// Wait for the command to finish
			if err := cmd.Wait(); err != nil {
				fmt.Printf("%s%s%s%s: %v\n", colorCode, host, ansiReset, padding, formatError(err))
			}
		}(host, idx)
	}

	wg.Wait()
	logrus.Debugf("Command executed on %d hosts.", totalHosts)
	return nil
}
