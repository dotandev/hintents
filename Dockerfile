# Stage 1: Build Rust simulator
FROM rust:alpine AS builder-rust

WORKDIR /app/simulator

# Install musl-dev for static linking
RUN apk add --no-cache musl-dev

# Copy Rust project files
COPY simulator/Cargo.toml simulator/Cargo.lock ./
COPY simulator/src ./src

# Build release binary (statically linked by default on Alpine)
RUN cargo build --release

# Stage 2: Build Go CLI
FROM golang:1.24-alpine AS builder-go

WORKDIR /app

# Copy Go dependency files
COPY go.mod go.sum ./
RUN go mod download

# Copy Go source
COPY . .

# Build Go binary statically
ENV CGO_ENABLED=0
RUN go build -o erst cmd/erst/main.go

# Stage 3: Final Runtime Image
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates

# Copy binaries from builders
COPY --from=builder-go /app/erst .
COPY --from=builder-rust /app/simulator/target/release/erst-sim ./simulator/target/release/erst-sim

# Expose if needed (not for CLI)
ENTRYPOINT ["./erst"]
