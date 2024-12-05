package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/innogames/polysh-go/pkg"
	"github.com/sirupsen/logrus"
)

func main() {
	// Parse command-line flags
	command := flag.String("command", "", "Command to execute on remote hosts")
	userFlag := flag.String("user", "", "Remote user to log in as")
	noColor := flag.Bool("no-color", false, "Disable colored hostnames")
	sshCmd := flag.String("ssh-cmd", "ssh", "SSH command to use for connecting")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	flag.Parse()

	hosts := flag.Args()

	if len(hosts) == 0 {
		fmt.Println("Usage: polysh [OPTIONS]... HOSTS...")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Set logrus log level
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05.000",
	})
	if *verbose {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}

	if *command != "" {
		// Execute the command directly on the hosts
		err := pkg.ExecuteCommandOnHosts(hosts, *command, *userFlag, *noColor, *verbose, *sshCmd)
		if err != nil {
			logrus.Error(err)
			os.Exit(1)
		}
	} else {
		// Enter interactive mode
		err := pkg.InteractiveMode(hosts, *userFlag, *noColor, *verbose, *sshCmd)
		if err != nil {
			logrus.Error(err)
			os.Exit(1)
		}
	}

	os.Exit(0)
}
