package pkg

import (
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
)

// StartInteractiveSession handles the interactive session logic
func StartInteractiveSession(hostSessions map[string]*HostSession, outputMutex *sync.Mutex) error {
	// Move to a new line before starting interaction
	outputMutex.Lock()
	fmt.Println()
	outputMutex.Unlock()

	// Create per-host output readers
	outputChan := make(chan hostOutput, 100) // Buffered channel to prevent blocking
	doneChan := make(chan struct{})
	var outputWg sync.WaitGroup

	// Start a goroutine to read output from all hosts
	for _, hs := range hostSessions {
		outputWg.Add(1)
		go func(hs *HostSession) {
			defer outputWg.Done()
			ReadHostOutput(hs, outputChan)
		}(hs)
	}

	// Start a goroutine to read user input
	inputChan := make(chan string)
	go ReadUserInput(inputChan, doneChan)

	// Main loop
	for {
		select {
		case input, ok := <-inputChan:
			// wait for user input
			if !ok {
				logrus.Errorf("Input channel closed, exiting")
				close(doneChan)
				outputWg.Wait()
				return nil
			}
			// exit -> close all connections and return
			if input == ":quit" || input == ":q" || input == ":exit" {
				logrus.Info("Exiting...")
				close(doneChan)
				close(inputChan)
				outputWg.Wait()
				return nil
			}

			// handle special control commands like :upload
			if len(input) > 0 && input[0] == ':' {
				HandleControlCommand(input, hostSessions)
				continue
			}

			// Send input to all hosts
			for _, hs := range hostSessions {
				_, err := hs.Stdin.Write([]byte(input + "\n"))
				if err != nil {
					logrus.Errorf("Failed to send command to host %s: %v", hs.Host, err)
				}
			}
		case output := <-outputChan:
			// we received output from a host -> print it!
			outputMutex.Lock()
			fmt.Printf("\r%s%s%s: %s\n", output.ColorCode, output.Host, reset, output.Data)
			// Reprint prompt
			fmt.Print("> ")
			outputMutex.Unlock()
		}
	}
}
