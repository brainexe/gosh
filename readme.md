# gosh

Go-based multi SSH session manager for parallel command execution.

## Features

- Execute commands on multiple hosts simultaneously
- Interactive mode with tab completion and command history
- Colored output for easy host identification
- SSH connection timeouts and error handling
- File upload capability (`:upload` command)
- Connection testing and verbose output
- Support for dynamic host discovery

## Installation

```sh
go install github.com/brainexe/gosh@latest
# or build from source
make build
```

## Usage

**Single command execution:**
```bash
gosh -c "uptime" server{1..3}
gosh -c "df -h" $(cat hosts.txt)
gosh -u user -c "df -h" web01 web02 db01
```

**Interactive mode:**
```bash
gosh server{1..3}
🖥️ [3]> uptime
web01: 14:23:15 up 45 days,  3:45,  1 user,  load average: 0.15, 0.08, 0.06
web02: 14:23:15 up 45 days,  3:45,  1 user,  load average: 0.12, 0.07, 0.05
web03: 14:23:15 up 45 days,  3:45,  1 user,  load average: 0.18, 0.09, 0.07
🖥️ [3]> ps aux | grep nginx
web01: root      1234  0.0  0.1  45678  2345 ?        Ss   10:30   0:00 nginx: master process /usr/sbin/nginx
web01: www-data  1235  0.0  0.0  45678  1234 ?        S    10:30   0:00 nginx: worker process
web02: root      1234  0.0  0.1  45678  2345 ?        Ss   10:30   0:00 nginx: master process /usr/sbin/nginx
web02: www-data  1235  0.0  0.0  45678  1234 ?        S    10:30   0:00 nginx: worker process
web03: root      1234  0.0  0.1  45678  2345 ?        Ss   10:30   0:00 nginx: master process /usr/sbin/nginx
web03: www-data  1235  0.0  0.0  45678  1234 ?        S    10:30   0:00 nginx: worker process
🖥️ [3]> :upload deploy.sh
web01: ✅ Upload successful: deploy.sh
web02: ✅ Upload successful: deploy.sh
web03: ✅ Upload successful: deploy.sh
🖥️ [3]> exit
```

**Dynamic host discovery:**
```bash
# Using command substitution for dynamic host lists
gosh $(cat hosts.txt)
🖥️ [5]> hostname
web01.local: web01.local
web02..local: web02.local
web03.local: web03.local
web04.local: web04.local
web05.local: web05.local
```

**Common examples:**
```bash
# Check disk space across web servers
gosh -c "df -h /" web{01..05}

# Restart service on multiple hosts
gosh -u admin -c "sudo systemctl restart nginx" web01 web02

# Monitor logs in real-time
gosh -c "tail -f /var/log/app.log" app{01..03}

# System health check
gosh --no-color -c "uptime && free -h" $(cat hosts.txt)

# Upload and execute scripts
gosh web01 web02
🖥️ [2]> :upload backup.sh
🖥️ [2]> chmod +x backup.sh && ./backup.sh

# Check system load and memory
gosh -v -c "uptime && free -h" prod{01..10}
```

## Interactive Commands

- `:upload <file>` - Upload file to all connected hosts
- `:hosts` - List all connected hosts
- `:help` - Show available commands
- `:exit`/`:quit` - Exit interactive mode
- `<command>` - Execute any command on all hosts

## Options

- `-c, --command` - Command to execute on all hosts
- `-u, --user` - SSH username (default: current user)
- `--no-color` - Disable colored output
- `-v, --verbose` - Enable verbose logging and connection testing
