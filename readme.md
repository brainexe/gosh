# gosh

Go-based multi SSH session manager for parallel command execution.

## Features

- Execute commands on multiple hosts simultaneously
- Interactive mode with command history
- Colored output for easy host identification
- SSH connection timeouts and error handling

## Installation

```sh
go install github.com/innogames/gosh@latest
# or build from source
make build
```

## Usage

**Single command execution:**
```bash
gosh -c "uptime" server1 server2 server3
gosh -u user -c "df -h" web01 web02 db01
```

**Interactive mode:**
```bash
gosh server1 server2
gosh> uptime
gosh> ps aux | grep nginx
gosh> exit
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
```

## Options

- `-c, --command` - Command to execute on all hosts
- `-u, --user` - SSH username (default: current user)
- `--no-color` - Disable colored output
- `-v, --verbose` - Enable verbose logging
