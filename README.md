# Eidolon

**Eidolon** (/aɪˈdoʊlən/) is Greek for a "phantom" or "double"—a proxy that stands in for a real binary to observe its behavior.

Eidolon is a transparent command-line interceptor. It executes any command as a proxy, siphoning its I/O (stdin/stdout/stderr) and metadata to a central eidolon-server for analysis without the host application knowing.

## How It Works

Eidolon works by sitting in front of a command in your PATH. When a program calls that command, it runs eidolon instead. Eidolon then:

1. Looks up the real binary to execute (from config or PATH)
2. Intercepts all I/O (stdin/stdout/stderr)
3. Passes through all data to the real command while buffering it
4. Captures command metadata (PID, PPID, arguments, exit code)
5. Sends all this data to the eidolon-server

This makes it ideal for observability scenarios where you want to monitor command execution without modifying the calling application.

### Use Cases

- **CI/CD Monitoring** - Capture all git operations in a CI pipeline
- **Debugging** - Log and analyze command execution in production
- **Audit Logging** - Keep a record of all CLI operations
- **GitLab/GitHub Integration** - Replace the git binary to monitor all repository operations

For example, in GitLab you could replace `/usr/bin/git` with a symlink to eidolon to capture every git operation performed by the GitLab runner.

## Setup

### 1. Build and Install

```bash
make
sudo make install
```

### 2. Configure Command Mapping

Create a directory in your PATH that will contain the intercepted commands:

```bash
mkdir -p ~/bin/intercepted
```

### 3. Replace Commands with Symlinks

For each command you want to intercept, create a symlink from the intercepted directory to eidolon:

```bash
ln -s $(which eidolon) ~/bin/intercepted/ls
ln -s $(which eidolon) ~/bin/intercepted/git
ln -s $(which eidolon) ~/bin/intercepted/docker
```

### 4. Configure Eidolon

Create `~/.config/eidolon/config.json` to specify which real binary to execute:

```json
{
  "server": "127.0.0.1:9999",
  "commands": {
    "ls": {
      "binary": "/bin/ls"
    },
    "git": {
      "binary": "/usr/bin/git"
    }
  }
}
```

### 5. Adjust PATH

Prepend your intercepted directory to PATH so eidolon is found first:

```bash
export PATH=~/bin/intercepted:$PATH
```

Add this to your `.bashrc` or `.zshrc` to make it permanent.

## Configuration

Eidolon uses a JSON configuration file located at `~/.config/eidolon/config.json`.

### Example Configuration

```json
{
  "server": "127.0.0.1:9999",
  "commands": {
    "git": {
      "binary": "/usr/bin/git-real",
      "env": {
        "GIT_TRACE": "1"
      },
      "flags": [
        {"from": ["-v", "--verbose"], "to": ["--debug"]}
      ]
    }
  }
}
```

### Configuration Options

| Option | Description |
|--------|-------------|
| `server` | Address of the eidolon-server (default: `127.0.0.1:9999`) |
| `commands.<name>.binary` | Absolute path to the real binary (optional - falls back to PATH lookup if not specified) |
| `commands.<name>.env` | Environment variables to set when running the command |
| `commands.<name>.flags` | Flag replacements applied to this command |

If `binary` is not specified for a command, eidolon will search for the original binary in PATH.

### Flag Replacements

The `flags` option allows you to replace command-line flags:

```json
{
  "flags": [
    {"from": ["--old-flag"], "to": ["--new-flag"]},
    {"from": ["-a", "-b"], "to": ["--combined"]}
  ]
}
```

## Example

### Terminal 1 - Start the server:
```bash
eidolon-server
```

### Terminal 2 - Run any command (e.g., `ls`):
```bash
ls -la
```

Since `ls` is now a symlink to eidolon in your PATH, the server will display:
```
[2024-01-15 10:30:45] PID: 1234 | PPID: 1233 | Alias: ls | Path: /bin/ls | Args: [-la] | Exit: 0
  stdout: total 40
drwxr-xr-x  5 user user 4096 Jan 15 10:30 .
...
```

## Building from Source

Requirements:
- Go 1.24+

```bash
make          # Build both binaries
make clean   # Clean build artifacts
make install # Install to /usr/local/bin
```

## License

MIT
