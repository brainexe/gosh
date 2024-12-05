package pkg

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

// ExecuteCommandOnHosts executes the given command on all hosts in parallel
func ExecuteCommandOnHosts(hosts []string, command string, userFlag string, noColor bool, verbose bool, sshCmd string) error {
	var wg sync.WaitGroup

	totalHosts := len(hosts)

	for idx, host := range hosts {
		wg.Add(1)
		go func(host string, idx int) {
			defer wg.Done()

			logrus.Debugf("Executing command on host %s", host)

			// Include the '-tt' option to force pseudo-terminal allocation
			sshArgs := []string{"-tt"}

			// Include verbosity flags if verbose mode is enabled
			if verbose {
				sshArgs = append(sshArgs, "-v")
			} else {
				sshArgs = append(sshArgs, "-o", "LogLevel=QUIET")
			}

			// Add user@host
			var userAtHost string
			if userFlag != "" {
				userAtHost = fmt.Sprintf("%s@%s", userFlag, host)
			} else {
				userAtHost = host
			}
			sshArgs = append(sshArgs, userAtHost)

			// Add the command, wrapped with 'bash -c' to ensure proper execution
			remoteCommand := fmt.Sprintf("bash -c %q", command)
			sshArgs = append(sshArgs, remoteCommand)

			// Prepare the SSH command
			cmd := exec.Command(sshCmd, sshArgs...)

			// If verbose, print the SSH command being executed
			if verbose {
				logrus.Debugf("SSH command: %s %s", sshCmd, strings.Join(sshArgs, " "))
			}

			// Capture stdout and stderr
			stdoutPipe, err := cmd.StdoutPipe()
			if err != nil {
				logrus.Errorf("Failed to get stdout for host %s: %v", host, err)
				return
			}

			stderrPipe, err := cmd.StderrPipe()
			if err != nil {
				logrus.Errorf("Failed to get stderr for host %s: %v", host, err)
				return
			}

			// Start the command
			if err := cmd.Start(); err != nil {
				logrus.Errorf("Failed to start SSH command for host %s: %v", host, err)
				return
			}

			// Reset color after hostname
			reset := "\033[0m"
			colorCode := ""
			if !noColor {
				colorCode = GetColorCode(idx)
			}

			// Create scanners for stdout and stderr
			stdoutScanner := bufio.NewScanner(stdoutPipe)
			stderrScanner := bufio.NewScanner(stderrPipe)

			// Wait group for scanning stdout and stderr
			var scanWg sync.WaitGroup
			scanWg.Add(2)

			// Channel to signal when output is done
			outputDone := make(chan struct{})

			// Scan stdout
			go func() {
				defer scanWg.Done()
				for stdoutScanner.Scan() {
					line := stdoutScanner.Text()
					fmt.Printf("%s%s%s: %s\n", colorCode, host, reset, line)
				}
			}()

			// Scan stderr
			go func() {
				defer scanWg.Done()
				for stderrScanner.Scan() {
					line := stderrScanner.Text()
					fmt.Printf("%s%s%s: %s\n", colorCode, host, reset, line)
				}
			}()

			// Wait for scanning to finish
			go func() {
				scanWg.Wait()
				close(outputDone)
			}()

			// Wait for command to finish
			if err := cmd.Wait(); err != nil {
				fmt.Printf("%s%s%s: %s\n", colorCode, host, reset, err)
			}

			// Wait for output scanning to finish
			<-outputDone

		}(host, idx)
	}

	wg.Wait()

	logrus.Debugf("Command executed on %d hosts.", totalHosts)
	return nil
}
