package pkg

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/sirupsen/logrus"
)

// HandleControlCommand handles control commands starting with ':'
func HandleControlCommand(input string, sessions map[string]*HostSession) {
	if len(input) < 2 {
		fmt.Println("Invalid control command.")
		return
	}

	commandParts := SplitCommand(input[1:])
	switch commandParts[0] {
	case "upload":
		if len(commandParts) < 2 {
			fmt.Println("Usage: :upload <file_path>")
			return
		}
		filePath := commandParts[1]
		UploadFileToHosts(filePath, sessions)
	default:
		fmt.Printf("Unknown control command: %s\n", commandParts[0])
	}
}

// SplitCommand splits the input command into parts
func SplitCommand(input string) []string {
	var parts []string
	var buf bytes.Buffer
	inQuotes := false

	for _, r := range input {
		switch r {
		case ' ':
			if inQuotes {
				buf.WriteRune(r)
			} else if buf.Len() > 0 {
				parts = append(parts, buf.String())
				buf.Reset()
			}
		case '"':
			inQuotes = !inQuotes
		default:
			buf.WriteRune(r)
		}
	}

	if buf.Len() > 0 {
		parts = append(parts, buf.String())
	}

	return parts
}

// UploadFileToHosts uploads a file to all connected hosts
func UploadFileToHosts(filePath string, sessions map[string]*HostSession) {
	var wg sync.WaitGroup

	for _, hs := range sessions {
		wg.Add(1)
		go func(hs *HostSession) {
			defer wg.Done()

			// todo aware of current dir
			remoteFileName := filepath.Base(filePath)

			scpArgs := []string{hs.Host + ":~/" + remoteFileName}
			scpArgs = append([]string{"-o", "LogLevel=QUIET"}, scpArgs...)

			logrus.Infof("Uploading file to host %s", hs.Host)

			cmd := exec.Command("scp", append(scpArgs, filePath)...)

			// Start the command
			output, err := cmd.CombinedOutput()
			if err != nil {
				logrus.Errorf("Failed to upload file to host %s: %v", hs.Host, err)
				logrus.Errorf("Output: %s", string(output))
				return
			}

			reset := "\033[0m"
			fmt.Printf("%s%s%s: Uploaded file %s\n", hs.ColorCode, hs.Host, reset, remoteFileName)
		}(hs)
	}
	wg.Wait()
}
