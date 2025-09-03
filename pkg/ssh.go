package pkg

import "fmt"

// getSSHArgs returns the arguments for the SSH command
func getSSHArgs(host string, userFlag string) []string {
	// Add user@host
	userAtHost := host
	if userFlag != "" {
		userAtHost = fmt.Sprintf("%s@%s", userFlag, host)
	}

	sshArgs := []string{
		"-tt",                  // Force pseudo-terminal allocation
		"-o", "LogLevel=QUIET", // Suppress warnings
		userAtHost, // user@host or just host
	}

	return sshArgs
}
