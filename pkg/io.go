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

// hostOutput represents the output from a host
type hostOutput struct {
	Host      string
	Data      string
	ColorCode string
	Padding   string
}

// readHostOutput reads the output from a host session and sends it to the output channel
func readHostOutput(hs *HostSession, outputChan chan<- hostOutput) {
	defer hs.Close()

	reader := bufio.NewReader(io.MultiReader(hs.Stdout, hs.Stderr))

	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			logrus.Errorf("Error reading from host %s: %v", hs.Host, err)
			break
		}

		if line != "" {
			// Remove ANSI escape sequences
			// todo needed, don't we want color here?
			cleanLine := stripAnsiEscapeSequences(line)
			cleanLine = strings.TrimSpace(cleanLine)

			// Ignore empty lines
			if cleanLine == "" {
				continue
			}

			// Debug: Output the received line
			logrus.Debugf("%s received: %s", hs.Host, cleanLine)

			outputChan <- hostOutput{
				Host:      hs.Host,
				Data:      cleanLine,
				ColorCode: hs.ColorCode,
				Padding:   hs.Padding,
			}
		}

		if err == io.EOF {
			break
		}
	}
}

// readUserInput reads user input from stdin and sends it to the input channel
func readUserInput(inputChan chan<- string, doneChan <-chan struct{}) {
	defer close(inputChan)

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		select {
		case <-doneChan:
			// application is shutting down
			return
		default:
			if scanner.Scan() {
				inputChan <- scanner.Text()
			} else {
				if err := scanner.Err(); err != nil {
					logrus.Errorf("Error reading user input: %v", err)
				}
				return
			}
		}
	}
}

// Regular expression to remove ANSI escape sequences
var ansiEscapeSequence = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripAnsiEscapeSequences(input string) string {
	return ansiEscapeSequence.ReplaceAllString(input, "")
}
