# mac-reporter

This repo contains a small Go client that reads the MAC address of a network interface and sends a JSON heartbeat over WebSocket every 5 seconds, and a simple WebSocket server that receives and logs the heartbeats.

Build and run

- Build the client and set the WebSocket URL and username at build time using `-ldflags`:

```bash
# Example: change websocket URL and username at compile time
go build -ldflags "-X 'main.websocketURL=ws://example.com:8080/ws' -X 'main.username=alice'" -o mac-reporter main.go

# Run the client (it will attempt to connect and send a heartbeat every 5s)
./mac-reporter
```

- Run the server (default listens on :8080 and handles `/ws`):

```bash
# from repository root
go run ./server
```

Notes
- Default `websocketURL` is `ws://localhost:8080/ws` and default `username` is `devtrung`.
- You can override both at build time using `-ldflags` as shown above. The variables are defined in the `main` package: `main.websocketURL` and `main.username`.
- If you prefer runtime flags or environment variables, swap the `var` declarations in `main.go` for `flag` parsing or `os.Getenv` usage.

Example compile for CI or multiple users

```bash
# Build for user bob pointing to a staging server
go build -ldflags "-X 'main.websocketURL=ws://staging.example.com:8080/ws' -X 'main.username=bob'" -o mac-reporter-bob main.go
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

