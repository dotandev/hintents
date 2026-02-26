#!/bin/bash
# Copyright 2025 Erst Users
# SPDX-License-Identifier: Apache-2.0

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
BINARY="./bin/generate-xdr-snapshot"
DEFAULT_COUNT=1000000
DEFAULT_OUTPUT="snapshot_1m.json"

# Functions
print_header() {
    echo -e "${BLUE}╔════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║  XDR Benchmark Snapshot Generator${NC}"
    echo -e "${BLUE}╚════════════════════════════════════════════╝${NC}"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

print_info() {
    echo -e "${YELLOW}ℹ${NC} $1"
}

check_binary() {
    if [ ! -f "$BINARY" ]; then
        print_error "Generator binary not found at $BINARY"
        print_info "Building generator..."
        go build -o "$BINARY" ./cmd/generate-xdr-snapshot
        if [ $? -eq 0 ]; then
            print_success "Generator built successfully"
        else
            print_error "Failed to build generator"
            exit 1
        fi
    fi
}

show_usage() {
    cat << EOF
${BLUE}Usage:${NC} $0 [COMMAND] [OPTIONS]

${BLUE}Commands:${NC}
  generate       Generate snapshot (default)
  test           Run tests for the generator
  bench          Run benchmarks
  clean          Clean generated files
  help           Show this help message

${BLUE}Options for 'generate':${NC}
  --count N      Number of entries (default: $DEFAULT_COUNT)
  --output FILE  Output file path (default: $DEFAULT_OUTPUT)
  --seed SEED    Random seed for reproducibility
  --quiet        Suppress progress output

${BLUE}Examples:${NC}
  # Generate default 1M entry snapshot
  $0 generate

  # Generate 100K entries for testing
  $0 generate --count 100000 --output test.json

  # Generate with reproducible seed
  $0 generate --count 1000000 --seed 12345

  # Generate 5M entries for stress testing
  $0 generate --count 5000000 --output snapshot_5m.json

  # Run tests
  $0 test

  # Run benchmarks
  $0 bench

  # Clean up generated files
  $0 clean

EOF
}

generate_snapshot() {
    local count=$DEFAULT_COUNT
    local output=$DEFAULT_OUTPUT
    local seed=""
    local verbose="true"

    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --count)
                count="$2"
                shift 2
                ;;
            --output)
                output="$2"
                shift 2
                ;;
            --seed)
                seed="$2"
                shift 2
                ;;
            --quiet)
                verbose="false"
                shift
                ;;
            *)
                print_error "Unknown option: $1"
                show_usage
                exit 1
                ;;
        esac
    done

    print_header
    print_info "Generating snapshot with $count entries"
    print_info "Output: $output"

    # Build command
    local cmd="$BINARY -count=$count -output=$output -verbose=$verbose"
    if [ -n "$seed" ]; then
        cmd="$cmd -seed=$seed"
        print_info "Seed: $seed"
    fi

    echo ""
    eval "$cmd"

    local status=$?
    if [ $status -eq 0 ]; then
        echo ""
        print_success "Snapshot generated successfully"

        # Show file info
        if [ -f "$output" ]; then
            local size=$(wc -c < "$output")
            local size_mb=$(echo "scale=2; $size / 1024 / 1024" | bc)
            print_info "File size: ${size_mb} MB ($size bytes)"
        fi
    else
        print_error "Failed to generate snapshot (exit code: $status)"
        exit 1
    fi
}

run_tests() {
    print_header
    print_info "Running tests..."
    echo ""

    go test -v -race ./cmd/generate-xdr-snapshot 2>&1 | head -200

    if [ $? -eq 0 ]; then
        print_success "Tests passed"
    else
        print_error "Tests failed"
        exit 1
    fi
}

run_benchmarks() {
    print_header
    print_info "Running benchmarks..."
    echo ""

    go test -bench=. -benchmem ./cmd/generate-xdr-snapshot

    if [ $? -eq 0 ]; then
        print_success "Benchmarks completed"
    else
        print_error "Benchmarks failed"
        exit 1
    fi
}

clean_files() {
    print_header
    print_info "Cleaning generated files..."

    local files=(
        "snapshot_*.json"
        "test_*.json"
        "bench_*.json"
        "demo.json"
        "large.json"
        "baseline.json"
    )

    local count=0
    for pattern in "${files[@]}"; do
        for file in $pattern; do
            if [ -f "$file" ]; then
                rm -f "$file"
                echo "  Removed: $file"
                ((count++))
            fi
        done
    done

    if [ $count -eq 0 ]; then
        print_info "No files to clean"
    else
        print_success "Cleaned $count file(s)"
    fi
}

# Main
main() {
    if [ $# -eq 0 ]; then
        check_binary
        generate_snapshot
    else
        case "$1" in
            generate)
                check_binary
                shift
                generate_snapshot "$@"
                ;;
            test)
                run_tests
                ;;
            bench)
                run_benchmarks
                ;;
            clean)
                clean_files
                ;;
            help)
                show_usage
                ;;
            *)
                print_error "Unknown command: $1"
                show_usage
                exit 1
                ;;
        esac
    fi
}

main "$@"
