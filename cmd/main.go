package main

import (
	"bufio"
	"fmt"
	"log"
	"os"

	"github.com/brainexe/gosh/pkg"
	"github.com/spf13/pflag"
)

func main() {
	// Parse flags
	command := pflag.StringP("command", "c", "", "Command to execute on all hosts")
	user := pflag.StringP("user", "u", "", "Username for SSH connections")
	noColor := pflag.Bool("no-color", false, "Disable colored output")
	verbose := pflag.BoolP("verbose", "v", false, "Enable verbose output")
	pflag.Parse()

	hosts := pflag.Args()
	if len(hosts) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] host1 [host2 ...]\n", os.Args[0])
		pflag.PrintDefaults()
		os.Exit(1)
	}

	if *verbose {
		log.SetOutput(os.Stderr)
	} else {
		log.SetOutput(bufio.NewWriter(os.Stderr))
	}

	if *command != "" {
		pkg.ExecuteCommand(hosts, *command, *user, *noColor)
	} else {
		pkg.InteractiveMode(hosts, *user, *noColor, *verbose)
	}
}
