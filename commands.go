package main

import (
	"fmt"
	"os"
	"sync"
)

// executeCommand runs a command on all hosts in parallel
func executeCommand(hosts []string, command, user string, noColor bool) {
	maxHostLen := maxLen(hosts)
	var wg sync.WaitGroup

	for i, host := range hosts {
		wg.Add(1)
		go func(host string, idx int) {
			defer wg.Done()
			runSSH(host, command, user, idx, maxHostLen, noColor)
		}(host, i)
	}

	wg.Wait()
}

// uploadFile uploads a file to all hosts in parallel
func uploadFile(hosts []string, filepath, user string, noColor bool) {
	// Check if local file exists
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		fmt.Printf("Error: File '%s' does not exist\n", filepath)
		return
	}

	maxHostLen := maxLen(hosts)
	var wg sync.WaitGroup

	for i, host := range hosts {
		wg.Add(1)
		go func(host string, idx int) {
			defer wg.Done()
			runSCP(host, filepath, user, idx, maxHostLen, noColor)
		}(host, i)
	}

	wg.Wait()
}
