package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/innogames/polysh-go/pkg"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

func main() {
	// Parse command-line flags
	command := flag.String("command", "", "Command to execute on remote hosts")
	userFlag := flag.String("user", "", "Remote user to log in as")
	noColor := flag.Bool("no-color", false, "Disable colored hostnames")
	sshCmd := flag.String("ssh-cmd", "ssh", "SSH command to use for connecting")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	flag.Parse()

	hosts := flag.Args()

	if len(hosts) == 0 {
		fmt.Println("Usage: polysh [OPTIONS]... HOSTS...")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Set logrus log level
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05.000",
	})
	if *verbose {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}

	// Create HostSessions
	hostSessions := make(map[string]*pkg.HostSession)
	var wg sync.WaitGroup
	var mutex sync.Mutex

	totalHosts := len(hosts)
	connectedHosts := 0

	progressChan := make(chan string)

	for i, host := range hosts {
		wg.Add(1)
		go func(host string, idx int) {
			defer wg.Done()
			hs, err := pkg.NewHostSession(host, *userFlag, idx, *noColor, *verbose, *sshCmd)
			if err != nil {
				logrus.Errorf("Failed to connect to host %s: %v", host, err)
				progressChan <- fmt.Sprintf("Failed to connect to host %s (%d/%d)", host, connectedHosts, totalHosts)
				return
			}
			mutex.Lock()
			hostSessions[host] = hs
			connectedHosts++
			mutex.Unlock()
		}(host, i)
	}

	// Start a goroutine to display progress
	go func() {
		for msg := range progressChan {
			fmt.Println(msg)
		}
	}()

	wg.Wait()
	close(progressChan)

	if len(hostSessions) == 0 {
		logrus.Error("No hosts connected successfully.")
		os.Exit(1)
	}

	fmt.Printf("Successfully connected to %d out of %d hosts.\n", connectedHosts, totalHosts)

	// If command is given, execute command on all hosts
	if *command != "" {
		executeCommandOnHosts(hostSessions, *command, *userFlag, *verbose)
		// Close sessions
		for _, hs := range hostSessions {
			hs.Close()
		}
		os.Exit(0)
	} else {
		// Enter interactive mode
		interactiveMode(hostSessions)
		// Close sessions
		for _, hs := range hostSessions {
			hs.Close()
		}
	}
}

func executeCommandOnHosts(sessions map[string]*pkg.HostSession, command string, userFlag string, verbose bool) {
	var wg sync.WaitGroup

	for _, hs := range sessions {
		wg.Add(1)
		go func(hs *pkg.HostSession) {
			defer wg.Done()

			logrus.Debugf("Executing command on host %s", hs.Host)

			// Include the '-tt' option to force pseudo-terminal allocation
			sshArgs := []string{"-tt"}

			// Include verbosity flags if verbose mode is enabled
			if verbose {
				// todo
				//		sshArgs = append(sshArgs, "-vvv")
			} else {
				sshArgs = append(sshArgs, "-o", "LogLevel=QUIET")
			}

			// Add user@host
			var userAtHost string
			if userFlag != "" {
				userAtHost = fmt.Sprintf("%s@%s", userFlag, hs.Host)
			} else {
				userAtHost = hs.Host
			}
			sshArgs = append(sshArgs, userAtHost)

			// Now, add the command, wrapped with 'bash -c' to ensure proper execution
			// todo which default shell
			remoteCommand := fmt.Sprintf("bash -c %q", command)
			sshArgs = append(sshArgs, remoteCommand)

			// Prepare the SSH command
			cmd := exec.Command("ssh", sshArgs...)

			// If verbose, print the SSH command being executed
			if verbose {
				logrus.Debugf("SSH command: ssh %s", strings.Join(sshArgs, " "))
			}

			// Capture stdout and stderr
			var stdoutBuf bytes.Buffer
			var stderrBuf bytes.Buffer
			cmd.Stdout = &stdoutBuf
			cmd.Stderr = &stderrBuf

			// Run the command
			err := cmd.Run()
			// Reset color after hostname
			reset := "\033[0m"

			// Collect output
			stdoutStr := stdoutBuf.String()
			stderrStr := stderrBuf.String()

			if verbose && stderrStr != "" {
				// Print SSH verbose output
				fmt.Printf("%s%s%s (SSH verbose output):\n%s", hs.ColorCode, hs.Host, reset, stderrStr)
			}

			if err != nil {
				logrus.Errorf("%s%s%s: Error executing command: %v", hs.ColorCode, hs.Host, reset, err)
			}

			stdoutStr = strings.TrimRight(stdoutStr, "\n")

			// Print stdout
			fmt.Printf("%s%s%s: %s\n", hs.ColorCode, hs.Host, reset, stdoutStr)

		}(hs)
	}

	wg.Wait()
}

func interactiveMode(sessions map[string]*pkg.HostSession) {
	// Create per-host output readers
	outputChan := make(chan pkg.HostOutput)
	doneChan := make(chan struct{})
	var wg sync.WaitGroup

	for _, hs := range sessions {
		wg.Add(1)
		go func(hs *pkg.HostSession) {
			defer wg.Done()
			pkg.ReadHostOutput(hs, outputChan)
		}(hs)
	}

	// Start a goroutine to read user input
	inputChan := make(chan string)
	go pkg.ReadUserInput(inputChan, doneChan)

	// Main loop
	for {
		select {
		case input, ok := <-inputChan:
			if !ok {
				logrus.Info("Input channel closed, exiting")
				close(doneChan)
				wg.Wait()
				return
			}
			if input == ":quit" {
				logrus.Info("Exiting")
				close(doneChan)
				wg.Wait()
				return
			}
			if len(input) > 0 && input[0] == ':' {
				pkg.HandleControlCommand(input, sessions)
				continue
			}
			// Send input to all hosts
			for _, hs := range sessions {
				_, err := hs.Stdin.Write([]byte(input + "\n"))
				if err != nil {
					logrus.Errorf("Failed to send command to host %s: %v", hs.Host, err)
				}
			}
		case output := <-outputChan:
			// Print output
			reset := "\033[0m"
			fmt.Printf("%s%s%s: %s", output.ColorCode, output.Host, reset, output.Data)
		}
	}
}
