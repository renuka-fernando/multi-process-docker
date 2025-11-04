# Multi-Process gRPC Docker with s6-overlay

This project demonstrates running two Go processes (gRPC server and client) in a single Alpine-based Docker container, managed by s6-overlay.

## Architecture

- **gRPC Server**: Listens on Unix Domain Socket (`/tmp/grpc.sock`)
- **gRPC Client**: Connects to server via UDS and makes periodic requests
- **Process Manager**: s6-overlay v3 manages both processes with proper supervision
- **Base Image**: Alpine Linux (minimal footprint)

## Features

- ✅ Two independent Go processes in one container
- ✅ Communication via Unix Domain Socket (UDS)
- ✅ Process supervision and automatic restart
- ✅ Graceful shutdown handling
- ✅ Proper dependency management (client depends on server)
- ✅ Multi-stage Docker build for minimal image size
- ✅ Stream and unary gRPC methods

## Project Structure

```
.
├── Dockerfile              # Multi-stage build with s6-overlay
├── Makefile               # Build and run commands
├── go.mod                 # Go module definition
├── proto/
│   └── service.proto      # gRPC service definition
├── server/
│   └── main.go           # gRPC server implementation
├── client/
│   └── main.go           # gRPC client implementation
└── s6-overlay/
    └── s6-rc.d/
        ├── grpc-server/   # Server service definition
        ├── grpc-client/   # Client service definition
        └── user/
            └── contents.d/ # Service bundle configuration
```

## Prerequisites

- Docker
- Make (optional, for convenience commands)

## Quick Start

### 1. Build the Docker image

```bash
make build
```

Or without Make:
```bash
docker build -t grpc-multiprocess .
```

### 2. Run the container

```bash
make run
```

Or without Make:
```bash
docker run -d --name grpc-container grpc-multiprocess
```

### 3. View logs

```bash
make logs
```

Or without Make:
```bash
docker logs -f grpc-container
```

## Available Make Commands

```bash
make help              # Show all available commands
make build             # Build the Docker image
make run               # Run container in detached mode
make run-interactive   # Run container in foreground
make stop              # Stop and remove container
make logs              # View all logs
make logs-server       # View server logs only
make logs-client       # View client logs only
make shell             # Open shell in running container
make clean             # Stop container and remove image
make rebuild           # Clean and rebuild
make restart           # Restart the container
```

## How It Works

### s6-overlay Process Management

s6-overlay v3 is a lightweight init system that:
- Manages multiple processes as "services"
- Automatically restarts crashed processes
- Handles proper shutdown order based on dependencies
- Provides proper signal handling and zombie reaping

### Service Definitions

Both processes are defined as s6 services:

**Server Service** (`s6-overlay/s6-rc.d/grpc-server/`):
- Type: `longrun` (long-running process)
- Runs: `/app/server`

**Client Service** (`s6-overlay/s6-rc.d/grpc-client/`):
- Type: `longrun`
- Runs: `/app/client`
- Depends on: `grpc-server`

### Communication Flow

1. Container starts → s6-overlay init system launches
2. Server process starts → Creates UDS at `/tmp/grpc.sock`
3. Client process starts → Connects to server via UDS
4. Client makes periodic requests:
   - `SayHello` every 5 seconds
   - `StreamMessages` every 3rd request (streams 5 messages)

## Logs Output Example

```
[INFO] Starting gRPC Server...
[INFO] gRPC Server listening on Unix Domain Socket: /tmp/grpc.sock
[INFO] Starting gRPC Client...
[INFO] Successfully connected to gRPC server via UDS

--- Request #1: SayHello ---
[Server] Received SayHello request from: Docker Client (request #1)
[Client] Response: Hello, Docker Client! Welcome to gRPC over UDS. (Server request count: 1)

--- Request #3: StreamMessages ---
[Server] Received StreamMessages request for 5 messages
[Client] Received: Stream message number 1 (index: 1)
[Client] Received: Stream message number 2 (index: 2)
...
```

## Development

### Local Proto Generation

If you want to generate protobuf code locally:

```bash
# Install protoc and plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Generate code
make proto
```

### Inspecting Running Container

```bash
# Open shell
make shell

# Check running processes
ps aux

# View s6 service status
s6-rc -a list

# Follow specific service logs (inside container)
tail -f /var/log/grpc-server/current
tail -f /var/log/grpc-client/current
```

## Why s6-overlay?

Compared to other solutions:

| Solution | Size | Complexity | Features |
|----------|------|------------|----------|
| **s6-overlay** | ~1MB | Medium | ✅ Supervision, ✅ Dependencies, ✅ Graceful shutdown |
| Shell script + tini | ~10KB | Low | ❌ No supervision, ❌ No dependencies |
| Supervisord | ~50MB+ | Low | ✅ Supervision, ❌ Python dependency |
| systemd | Large | High | ✅ Full init system (overkill) |

s6-overlay provides the best balance of features and footprint for this use case.

## Troubleshooting

### Container exits immediately
- Check logs: `docker logs grpc-container`
- Run interactively: `make run-interactive`
- Verify s6 service definitions are correct

### Client can't connect to server
- Server should start before client (dependency defined)
- Check socket exists: `docker exec grpc-container ls -la /tmp/grpc.sock`
- View server logs: `make logs-server`

### Build fails
- Ensure Docker is running
- Check Go version compatibility in Dockerfile
- Verify all source files are present

## License

MIT
