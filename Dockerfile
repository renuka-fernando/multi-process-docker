# Stage 1: Build the Go binaries
FROM golang:alpine AS builder

# Install required tools
RUN apk add --no-cache git protobuf-dev

# Install protoc-gen-go and protoc-gen-go-grpc
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Generate protobuf code
RUN protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    proto/service.proto

# Build server
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ./server-bin ./server

# Build client
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ./client-bin ./client

# Stage 2: Create final image with s6-overlay
FROM alpine:3.19

# Install s6-overlay
ARG S6_OVERLAY_VERSION=3.1.6.2
ENV S6_OVERLAY_VERSION=${S6_OVERLAY_VERSION}

ADD https://github.com/just-containers/s6-overlay/releases/download/v${S6_OVERLAY_VERSION}/s6-overlay-noarch.tar.xz /tmp
RUN tar -C / -Jxpf /tmp/s6-overlay-noarch.tar.xz && rm /tmp/s6-overlay-noarch.tar.xz

ADD https://github.com/just-containers/s6-overlay/releases/download/v${S6_OVERLAY_VERSION}/s6-overlay-x86_64.tar.xz /tmp
RUN tar -C / -Jxpf /tmp/s6-overlay-x86_64.tar.xz && rm /tmp/s6-overlay-x86_64.tar.xz

# Create app directory
RUN mkdir -p /app

# Copy binaries from builder
COPY --from=builder /build/server-bin /app/server
COPY --from=builder /build/client-bin /app/client

# Copy s6-overlay service definitions
COPY s6-overlay/s6-rc.d /etc/s6-overlay/s6-rc.d

# Make binaries and run scripts executable
RUN chmod +x /app/server && \
    chmod +x /app/client && \
    chmod +x /etc/s6-overlay/s6-rc.d/grpc-server/run && \
    chmod +x /etc/s6-overlay/s6-rc.d/grpc-client/run

# s6-overlay requires /init as the entrypoint
ENTRYPOINT ["/init"]
