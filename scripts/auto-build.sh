#!/bin/bash

# Auto-build script for docker-notify
# Automatically detects the current architecture and builds the appropriate Docker image

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

# Detect current architecture
detect_architecture() {
    local arch=$(uname -m)
    case $arch in
        x86_64)
            echo "linux/amd64"
            ;;
        aarch64|arm64)
            echo "linux/arm64"
            ;;
        armv7l)
            echo "linux/arm/v7"
            ;;
        *)
            print_error "Unsupported architecture: $arch"
            exit 1
            ;;
    esac
}

# Main script
main() {
    print_info "Docker Notify Auto-Build Script"
    echo ""

    # Detect architecture
    print_step "Detecting system architecture..."
    ARCH=$(uname -m)
    PLATFORM=$(detect_architecture)
    print_info "Detected architecture: $ARCH"
    print_info "Docker platform: $PLATFORM"
    echo ""

    # Check if Docker is available
    print_step "Checking Docker availability..."
    if ! command -v docker &> /dev/null; then
        print_error "Docker is not installed or not in PATH"
        exit 1
    fi

    if ! docker info &> /dev/null; then
        print_error "Docker daemon is not running"
        exit 1
    fi
    print_info "Docker is available and running"
    echo ""

    # Build the image
    print_step "Building Docker image for $PLATFORM..."
    IMAGE_NAME="docker-notify:latest"

    if docker build --platform "$PLATFORM" -t "$IMAGE_NAME" .; then
        print_info "Build completed successfully!"
        echo ""

        # Show image info
        print_step "Image information:"
        docker images | grep docker-notify | head -1
        echo ""

        print_info "To run the container:"
        echo "  docker run --rm -v /var/run/docker.sock:/var/run/docker.sock $IMAGE_NAME"
        echo ""

        print_info "To run with docker-compose:"
        echo "  docker-compose up -d"
        echo ""

    else
        print_error "Build failed!"
        exit 1
    fi
}

# Run main function
main "$@"
