# Docker Multi-Architecture Support

## Overview

The `erst` project provides Docker images for both `amd64` and `arm64` architectures, containing:

- Go CLI binary (`erst`)
- Rust simulator binary (`erst-sim`)

Both binaries are seamlessly linked and ready to use in the container.

## Supported Architectures

- `linux/amd64` - x86_64 processors (Intel/AMD)
- `linux/arm64` - ARM64 processors (Apple Silicon, AWS Graviton, etc.)

Docker automatically pulls the correct image for your platform.

## Quick Start

### Pull the Latest Image

```bash
# Pull latest from GitHub Container Registry
docker pull ghcr.io/dotandev/hintents:latest

# Or pull a specific version
docker pull ghcr.io/dotandev/hintents:v1.0.0
```

### Run the Container

```bash
# Show help
docker run --rm ghcr.io/dotandev/hintents:latest --help

# Show version
docker run --rm ghcr.io/dotandev/hintents:latest --version

# Run with configuration
docker run --rm -v $(pwd)/erst.toml:/app/erst.toml:ro \
  ghcr.io/dotandev/hintents:latest [command]
```

## Building Locally

### Build for Your Platform

```bash
# Build for current platform
docker build -t erst:local .

# Test the build
docker run --rm erst:local --version
```

### Build Multi-Architecture Images

```bash
# Set up buildx (one-time setup)
docker buildx create --name multiarch --use
docker buildx inspect --bootstrap

# Build for both architectures
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --tag erst:multiarch \
  --load \
  .
```

### Using Docker Compose

```bash
# Build and run
docker-compose up erst

# Build only
docker-compose build

# With Jaeger tracing
docker-compose --profile tracing up
```

## Image Details

### Image Layers

1. **Builder Stage (Rust)**: Compiles the Rust simulator
2. **Builder Stage (Go)**: Compiles the Go CLI
3. **Runtime Stage**: Minimal Alpine Linux with both binaries

### Image Size

- Approximate size: 50-80 MB (compressed)
- Base: Alpine Linux (minimal footprint)
- Static binaries (no runtime dependencies)

### Included Binaries

- `/app/erst` - Go CLI binary
- `/app/simulator/target/release/erst-sim` - Rust simulator binary

## CI/CD Integration

### Automated Builds

Images are automatically built and pushed on:

- Push to `main`/`master` branch → `latest` tag
- Pull requests → `pr-<number>` tag (not pushed)
- Version tags (`v*`) → versioned tags (`v1.0.0`, `1.0`, `1`)
- Commit SHA → `<branch>-<sha>` tag

### GitHub Actions Workflow

The `.github/workflows/docker-build.yml` workflow:

1. Sets up QEMU for multi-architecture emulation
2. Configures Docker Buildx
3. Builds for `linux/amd64` and `linux/arm64`
4. Pushes to GitHub Container Registry
5. Tests both architecture images
6. Generates build attestations

## Advanced Usage

### Specify Platform Explicitly

```bash
# Force amd64
docker pull --platform linux/amd64 ghcr.io/dotandev/hintents:latest

# Force arm64
docker pull --platform linux/arm64 ghcr.io/dotandev/hintents:latest
```

### Inspect Image Architecture

```bash
# View manifest
docker manifest inspect ghcr.io/dotandev/hintents:latest

# Check local image
docker image inspect ghcr.io/dotandev/hintents:latest | grep Architecture
```

### Run with Custom Entrypoint

```bash
# Access shell
docker run --rm -it --entrypoint sh ghcr.io/dotandev/hintents:latest

# Run simulator directly
docker run --rm --entrypoint /app/simulator/target/release/erst-sim \
  ghcr.io/dotandev/hintents:latest --help
```

## Development

### Local Testing

```bash
# Build locally
docker build -t erst:test .

# Run tests in container
docker run --rm erst:test sh -c "cd /app && go test ./..."

# Interactive development
docker run --rm -it -v $(pwd):/workspace erst:test sh
```

### Build Arguments

The Dockerfile accepts these build arguments:

```bash
docker build \
  --build-arg VERSION=1.0.0 \
  --build-arg COMMIT_SHA=$(git rev-parse HEAD) \
  --build-arg BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
  -t erst:custom .
```

### Debugging Build Issues

```bash
# Build with verbose output
docker build --progress=plain --no-cache -t erst:debug .

# Build specific stage
docker build --target builder-go -t erst:go-builder .
docker build --target builder-rust -t erst:rust-builder .

# Inspect intermediate layers
docker run --rm -it erst:go-builder sh
```

## Registry Configuration

### GitHub Container Registry (GHCR)

Images are published to: `ghcr.io/dotandev/hintents`

#### Authentication

```bash
# Login with GitHub token
echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin

# Or use GitHub CLI
gh auth token | docker login ghcr.io -u USERNAME --password-stdin
```

### Alternative Registries

To push to Docker Hub or other registries:

```bash
# Docker Hub
docker tag erst:local username/erst:latest
docker push username/erst:latest

# AWS ECR
aws ecr get-login-password --region region | docker login --username AWS --password-stdin account.dkr.ecr.region.amazonaws.com
docker tag erst:local account.dkr.ecr.region.amazonaws.com/erst:latest
docker push account.dkr.ecr.region.amazonaws.com/erst:latest
```

## Troubleshooting

### Build Fails on ARM64

If Rust build fails on ARM64:

```bash
# Ensure QEMU is installed
docker run --rm --privileged multiarch/qemu-user-static --reset -p yes

# Verify QEMU
docker buildx ls
```

### Image Size Too Large

Optimize by:

1. Using `.dockerignore` to exclude unnecessary files
2. Multi-stage builds (already implemented)
3. Static linking (already enabled)
4. Stripping debug symbols: `-ldflags="-s -w"`

### Cross-Platform Issues

If binaries don't work on target platform:

```bash
# Verify binary architecture
docker run --rm --entrypoint file ghcr.io/dotandev/hintents:latest /app/erst

# Check for dynamic linking issues
docker run --rm --entrypoint ldd ghcr.io/dotandev/hintents:latest /app/erst
```

## Security

### Image Scanning

```bash
# Scan with Docker Scout
docker scout cves ghcr.io/dotandev/hintents:latest

# Scan with Trivy
trivy image ghcr.io/dotandev/hintents:latest
```

### Best Practices

- Images use minimal Alpine base
- No root user required
- Static binaries (no dynamic dependencies)
- Regular security updates via CI/CD
- Build attestations for provenance

## Performance

### Build Cache

The workflow uses GitHub Actions cache:

- Speeds up subsequent builds
- Caches Go modules and Cargo dependencies
- Reduces build time by 50-70%

### Pull Performance

```bash
# Pre-pull for faster startup
docker pull ghcr.io/dotandev/hintents:latest

# Use in Kubernetes with imagePullPolicy: IfNotPresent
```

## References

- [Docker Buildx Documentation](https://docs.docker.com/buildx/working-with-buildx/)
- [Multi-platform Images](https://docs.docker.com/build/building/multi-platform/)
- [GitHub Container Registry](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)
