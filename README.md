# mac-reporter

This repo contains a small Go client that reads the MAC address of the first wireless interface (wl*) and sends a JSON heartbeat over WebSocket every 5 seconds, and a simple WebSocket server that receives and logs the heartbeats.

Configuration

The client reads configuration from `/root/dtth.conf` using cleanenv. See `dtth.conf.example` for the format:

```
SERVER_URL=ws://192.168.10.140:8080/ws
SBD=123
```

- `SERVER_URL`: WebSocket endpoint to connect to  
- `SBD`: Student ID (sent as "username" in heartbeat payloads)

The config file is **deleted immediately after being read** by the client.

Build and run

- Build the client (static recommended):

```bash
# Static build
CGO_ENABLED=0 go build -ldflags "-s -w" -o mac-reporter main.go

# Run the client
# Note: requires /root/dtth.conf to exist
./mac-reporter
```

- Run the server (default listens on :8080 and handles `/ws`):

```bash
# from repository root
go run ./server
```

Static build and cross-compiling

- To build a statically-linked client binary on Linux (recommended for simple deployment):

```bash
# Produce a static binary (disable cgo)
CGO_ENABLED=0 go build -ldflags "-s -w" -o mac-reporter-static main.go
```

- If you are building from a non-Linux host (for example macOS or Windows) and want a Linux/amd64 static binary, set `GOOS`/`GOARCH` and disable CGO:

```bash
# Cross-compile for linux/amd64 with CGO disabled
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w" -o mac-reporter-linux-amd64 main.go
```

- Notes on fully static musl builds or when CGO is required:
	- If a dependency requires CGO or you need a musl-linked binary, you'll need a cross-compiler toolchain (e.g., `musl-gcc`) and may set `CC` and enable `CGO_ENABLED=1`, then pass `-extldflags '-static'` to `-ldflags`.
	- Building musl-static binaries from macOS/Windows is often easier using a Docker build image (e.g., `golang:1.20` + `musl` toolchain) or a CI runner running Linux.

