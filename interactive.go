package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// interactiveMode starts an interactive session
func interactiveMode(hosts []string, user string, noColor bool) {
	fmt.Println("Interactive mode - type commands to execute on all hosts")
	fmt.Println("Type 'exit' or 'quit' to exit, 'help' for help")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("gosh> ")
		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		switch {
		case line == "exit" || line == "quit":
			return
		case line == "help":
			showHelp()
		case strings.HasPrefix(line, ":upload "):
			filepath := strings.TrimSpace(strings.TrimPrefix(line, ":upload"))
			if filepath == "" {
				fmt.Println("Usage: :upload <filepath>")
				continue
			}
			uploadFile(hosts, filepath, user, noColor)
		default:
			executeCommand(hosts, line, user, noColor)
		}
	}
}

// showHelp displays help information
func showHelp() {
	fmt.Println("Commands:")
	fmt.Println("  help       - Show this help")
	fmt.Println("  exit/quit  - Exit interactive mode")
	fmt.Println("  :upload <file>    - Upload file to all hosts (current directory)")
	fmt.Println("  <command>  - Execute command on all hosts")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  date       - Show date/time on all hosts")
	fmt.Println("  uptime     - Show uptime on all hosts")
	fmt.Println("  ls -la     - List files on all hosts")
	fmt.Println("  :upload script.sh - Upload script.sh to all hosts")
}
