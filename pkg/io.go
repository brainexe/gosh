package pkg

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
)

// HostOutput represents the output from a host
type HostOutput struct {
	Host      string
	Data      string
	ColorCode string
}

func ReadHostOutput(hs *HostSession, outputChan chan<- HostOutput) {
	defer hs.Close()
	reader := bufio.NewReader(io.MultiReader(hs.Stdout, hs.Stderr))
	var buffer string

	for {
		data, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			logrus.Errorf("Error reading from host %s: %v", hs.Host, err)
			break
		}

		if data != "" {
			// Remove ANSI escape sequences
			data = stripAnsiEscapeSequences(data)

			// Ignore "Last login" lines
			if strings.HasPrefix(data, "Last login") {
				// todo better ssh/shell flag to ignore?
				continue
			}

			buffer += data

			// Check if we have received the prompt marker
			if strings.Contains(buffer, hs.PromptMarker) {
				// Split the buffer at the prompt marker
				parts := strings.Split(buffer, hs.PromptMarker)
				if len(parts) > 0 {
					commandOutput := parts[0]
					// Remove any trailing newlines
					commandOutput = strings.TrimRight(commandOutput, "\r\n")

					// Split command output into lines
					lines := strings.Split(commandOutput, "\n")

					// Output each line prefixed with hostname and color
					for _, line := range lines {
						line = strings.TrimSpace(line)
						if line == "" {
							continue
						}
						outputChan <- HostOutput{
							Host:      hs.Host,
							Data:      line,
							ColorCode: hs.ColorCode,
						}
					}
				}
				// Reset buffer
				buffer = ""
			}
		}

		if err == io.EOF {
			break
		}
	}
}

// ReadUserInput reads user input and sends it to the input channel
func ReadUserInput(inputChan chan<- string, doneChan <-chan struct{}) {
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		select {
		case <-doneChan:
			close(inputChan)
			return
		default:
			if scanner.Scan() {
				input := scanner.Text()
				inputChan <- input
			} else {
				if err := scanner.Err(); err != nil {
					logrus.Errorf("Error reading user input: %v", err)
				}
				close(inputChan)
				return
			}
		}
	}
}

var ansiEscapeSequence = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripAnsiEscapeSequences(input string) string {
	return ansiEscapeSequence.ReplaceAllString(input, "")
}
