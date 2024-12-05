package pkg

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"sync"
)

// InteractiveMode starts an interactive session with all hosts
func InteractiveMode(hosts []string, userFlag string, noColor bool, verbose bool, sshCmd string) error {
	// Create HostSessions
	hostSessions := make(map[string]*HostSession)
	var wg sync.WaitGroup
	var mutex sync.Mutex
	var outputMutex sync.Mutex // Added mutex for synchronized output

	totalHosts := len(hosts)
	connectedHosts := 0

	// Progress channel to receive connection updates
	progressChan := make(chan struct{})

	// Start a goroutine to monitor connection progress
	go func() {
		for range progressChan {
			outputMutex.Lock()
			if connectedHosts == totalHosts {
				fmt.Printf("\rReady (%d) ", totalHosts)
			} else {
				fmt.Printf("\rConnecting (%d/%d)> ", connectedHosts, totalHosts)
			}
			outputMutex.Unlock()
		}
	}()

	// Initial connecting message
	outputMutex.Lock()
	fmt.Printf("Connecting (0/%d)> ", totalHosts)
	outputMutex.Unlock()

	// Start a goroutine to connect to each host
	for i, host := range hosts {
		wg.Add(1)
		go func(host string, idx int) {
			// TODO: throttle new connections if needed
			defer wg.Done()
			hs, err := NewHostSession(host, userFlag, idx, noColor, sshCmd)
			if err != nil {
				logrus.Errorf("Failed to connect to host %s: %v", host, err)
				return
			}
			mutex.Lock()
			hostSessions[host] = hs
			connectedHosts++
			mutex.Unlock()
			progressChan <- struct{}{}
		}(host, i)
	}

	// Wait for all connections to complete
	wg.Wait()
	close(progressChan)

	if len(hostSessions) == 0 {
		logrus.Error("No hosts connected successfully.")
		return fmt.Errorf("no hosts connected successfully")
	}

	// Enter interactive session
	err := StartInteractiveSession(hostSessions, &outputMutex)
	if err != nil {
		logrus.Errorf("Error during interactive session: %v", err)
	}

	// Close all sessions
	for _, hs := range hostSessions {
		if err := hs.Close(); err != nil {
			logrus.Errorf("Error closing host session for %s: %v", hs.Host, err)
		}
	}

	return nil
}
