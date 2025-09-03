package pkg

import (
	"errors"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
)

// todo threadsafe fmt.Println

// InteractiveMode starts an interactive session with all hosts
func InteractiveMode(hosts []string, userFlag string, noColor bool, sshCmd string) error {
	// Create HostSessions
	hostSessions := make(map[string]*HostSession)
	var wg sync.WaitGroup
	var mutex sync.Mutex
	var outputMutex sync.Mutex // Added mutex for synchronized output

	totalHosts := len(hosts)
	connectedHosts := 0

	// Progress channel to receive connection updates
	progressChan := make(chan int, 1)
	// first signal for progress bar
	progressChan <- 0

	// Start a goroutine to monitor connection progress
	go func() {
		for connected := range progressChan {
			outputMutex.Lock()
			if connected == totalHosts {
				fmt.Printf("\rReady (%d)", connected)
			} else {
				fmt.Printf("\rConnecting (%d/%d)>", connected, totalHosts)
			}
			outputMutex.Unlock()
		}
	}()

	maxHostLen := maxHostName(hosts)
	// Start a goroutine to connect to each host
	for i, host := range hosts {
		wg.Add(1)
		go func(host string, idx int) {
			defer wg.Done()
			hs, err := newHostSession(host, userFlag, idx, noColor, sshCmd, maxHostLen)
			if err != nil {
				logrus.Errorf("Failed to connect to host %s: %v", host, err)
				return
			}
			mutex.Lock()
			hostSessions[host] = hs
			connectedHosts++
			progressChan <- connectedHosts
			mutex.Unlock()
		}(host, i)
	}

	// Wait for all connections to complete
	wg.Wait()
	close(progressChan)
	// Move to a new line before starting interaction
	fmt.Println()

	if len(hostSessions) == 0 {
		return errors.New("no hosts connected successfully")
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
