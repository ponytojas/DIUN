#!/bin/bash

# Multi-architecture Docker build script for docker-notify
# This script builds the Docker image for multiple architectures

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Default values
IMAGE_NAME="docker-notify"
TAG="latest"
PUSH=false
PLATFORMS="linux/amd64,linux/arm64,linux/arm/v7"

# Function to print colored output
print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to show usage
usage() {
    echo "Usage: $0 [OPTIONS]"
    echo "Build multi-architecture Docker images for docker-notify"
    echo ""
    echo "Options:"
    echo "  -n, --name NAME       Image name (default: docker-notify)"
    echo "  -t, --tag TAG         Image tag (default: latest)"
    echo "  -p, --push            Push images to registry"
    echo "  --platforms PLATFORMS Comma-separated list of platforms"
    echo "                        (default: linux/amd64,linux/arm64,linux/arm/v7)"
    echo "  -h, --help            Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0                                    # Build for default platforms"
    echo "  $0 -t v1.0.0                        # Build with specific tag"
    echo "  $0 -n myregistry/docker-notify -p   # Build and push to registry"
    echo "  $0 --platforms linux/amd64,linux/arm64  # Build for specific platforms"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -n|--name)
            IMAGE_NAME="$2"
            shift 2
            ;;
        -t|--tag)
            TAG="$2"
            shift 2
            ;;
        -p|--push)
            PUSH=true
            shift
            ;;
        --platforms)
            PLATFORMS="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

# Check if Docker buildx is available
if ! docker buildx version >/dev/null 2>&1; then
    print_error "Docker buildx is not available. Please install Docker Desktop or enable buildx."
    exit 1
fi

# Create buildx builder if it doesn't exist
BUILDER_NAME="multiarch-builder"
if ! docker buildx inspect "$BUILDER_NAME" >/dev/null 2>&1; then
    print_status "Creating buildx builder: $BUILDER_NAME"
    docker buildx create --name "$BUILDER_NAME" --driver docker-container --bootstrap
fi

# Use the builder
print_status "Using buildx builder: $BUILDER_NAME"
docker buildx use "$BUILDER_NAME"

# Build command
BUILD_CMD="docker buildx build"
BUILD_CMD="$BUILD_CMD --platform $PLATFORMS"
BUILD_CMD="$BUILD_CMD --tag $IMAGE_NAME:$TAG"

if [ "$PUSH" = true ]; then
    BUILD_CMD="$BUILD_CMD --push"
    print_status "Building and pushing $IMAGE_NAME:$TAG for platforms: $PLATFORMS"
else
    BUILD_CMD="$BUILD_CMD --load"
    print_status "Building $IMAGE_NAME:$TAG for platforms: $PLATFORMS"
    print_warning "Note: --load only supports single platform. Use --push for multi-platform."
fi

BUILD_CMD="$BUILD_CMD ."

# Execute build
print_status "Executing: $BUILD_CMD"
eval $BUILD_CMD

if [ $? -eq 0 ]; then
    print_status "Build completed successfully!"

    if [ "$PUSH" = true ]; then
        print_status "Images pushed to registry."
    else
        print_status "Images built locally."
        print_status "To inspect the image:"
        echo "  docker run --rm $IMAGE_NAME:$TAG --version"
    fi

    print_status "To run the container:"
    echo "  docker run --rm -v /var/run/docker.sock:/var/run/docker.sock $IMAGE_NAME:$TAG"
else
    print_error "Build failed!"
    exit 1
fi

# Show final images
print_status "Available images:"
docker images | grep "$IMAGE_NAME" | head -5
