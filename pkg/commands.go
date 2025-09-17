package pkg

import (
	"fmt"
	"os"
	"sync"
)

// ExecuteCommand runs a command on all hosts in parallel
func ExecuteCommand(hosts []string, command, user string, noColor bool) {
	maxHostLen := MaxLen(hosts)
	var wg sync.WaitGroup

	for i, host := range hosts {
		wg.Add(1)
		go func(host string, idx int) {
			defer wg.Done()
			RunSSH(host, command, user, idx, maxHostLen, noColor)
		}(host, i)
	}

	wg.Wait()
}

// UploadFile uploads a file to all hosts in parallel
func UploadFile(hosts []string, filepath, user string, noColor bool) {
	// Check if local file exists
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		fmt.Printf("Error: File '%s' does not exist\n", filepath)
		return
	}

	maxHostLen := MaxLen(hosts)
	var wg sync.WaitGroup

	for i, host := range hosts {
		wg.Add(1)
		go func(host string, idx int) {
			defer wg.Done()
			RunSCP(host, filepath, user, idx, maxHostLen, noColor)
		}(host, i)
	}

	wg.Wait()
}
