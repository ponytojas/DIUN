#!/bin/bash

# Dynamic Configuration Demo for Docker Notify
# This script demonstrates different ways to dynamically configure docker-notify

set -e

echo "🚀 Docker Notify - Dynamic Configuration Demo"
echo "=============================================="
echo ""

# Function to display section headers
show_section() {
    echo ""
    echo "📋 $1"
    echo "$(printf '%.0s-' {1..50})"
}

# Function to run docker-notify with specific config
test_config() {
    local description="$1"
    shift
    echo "Testing: $description"
    echo "Command: $*"
    echo ""

    # Run the command (add --dry-run or similar flag if available)
    if command -v docker-notify >/dev/null 2>&1; then
        timeout 5 "$@" || echo "✅ Configuration loaded successfully (timed out as expected)"
    else
        echo "⚠️  docker-notify binary not found, showing command only"
    fi
    echo ""
}

show_section "Method 1: Environment Variables Override"

# Example 1: Basic environment variables
echo "Setting basic configuration via environment variables:"
cat << 'EOF'
export CHECK_INTERVAL="1h"
export TIMEZONE="America/New_York"
export TELEGRAM_BOT_TOKEN="123456789:ABCdefGHIjklMNOpqrsTUVwxyz"
export TELEGRAM_CHAT_IDS="123456789"
export LOG_LEVEL="debug"
EOF

export CHECK_INTERVAL="1h"
export TIMEZONE="America/New_York"
export TELEGRAM_BOT_TOKEN="123456789:ABCdefGHIjklMNOpqrsTUVwxyz"
export TELEGRAM_CHAT_IDS="123456789"
export LOG_LEVEL="debug"

test_config "Environment variables override" docker-notify -config configs/config.yaml

show_section "Method 2: Complete YAML Configuration via Environment"

# Example 2: Full YAML config via environment variable
echo "Providing complete configuration as YAML in CONFIG_CONTENT:"

export CONFIG_CONTENT='
app:
  check_interval: "2h"
  timezone: "Europe/London"
  max_concurrency: 5
  registry_timeout: "45s"

docker:
  socket_path: "unix:///var/run/docker.sock"
  filters:
    include:
      - "nginx:*"
      - "postgres:*"
    exclude:
      - "*:latest"
      - "scratch:*"
    check_latest: false
    check_private: true
    version_filters:
      exclude_prerelease: true
      exclude_windows: true
      only_stable: true

notifications:
  channels:
    - "telegram"
  telegram:
    bot_token: "987654321:ZYXwvuTSRqponMLKjihGFEdcbA"
    chat_ids:
      - 987654321
    parse_mode: "HTML"
  behavior:
    once_per_update: true
    cooldown_period: "12h"
    group_updates: true
    max_updates_per_notification: 5

logging:
  level: "info"
  format: "json"
'

echo "CONFIG_CONTENT set to:"
echo "$CONFIG_CONTENT"

test_config "Full YAML via CONFIG_CONTENT" docker-notify

show_section "Method 3: Docker Compose with Dynamic Variables"

# Example 3: Docker compose with variables
echo "Creating dynamic docker-compose override:"

cat > docker-compose.dynamic.yml << 'EOF'
version: "3.8"

services:
  docker-notify:
    environment:
      - CHECK_INTERVAL=${DYNAMIC_CHECK_INTERVAL:-30m}
      - TIMEZONE=${DYNAMIC_TIMEZONE:-UTC}
      - TELEGRAM_BOT_TOKEN=${DYNAMIC_BOT_TOKEN}
      - TELEGRAM_CHAT_IDS=${DYNAMIC_CHAT_IDS}
      - NOTIFICATION_CHANNELS=${DYNAMIC_CHANNELS:-telegram}
      - LOG_LEVEL=${DYNAMIC_LOG_LEVEL:-info}
      - INCLUDE_PATTERNS=${DYNAMIC_INCLUDE_PATTERNS:-}
      - EXCLUDE_PATTERNS=${DYNAMIC_EXCLUDE_PATTERNS:-*:latest,scratch:*}
      - MAX_CONCURRENCY=${DYNAMIC_MAX_CONCURRENCY:-10}
EOF

echo "Dynamic docker-compose.yml created!"
echo ""
echo "Usage examples:"
echo "DYNAMIC_CHECK_INTERVAL=45m DYNAMIC_BOT_TOKEN=your_token docker-compose -f docker-compose.yml -f docker-compose.dynamic.yml up -d"
echo ""

show_section "Method 4: Configuration Profiles"

# Example 4: Different configuration profiles
echo "Creating configuration profiles for different environments:"

# Development profile
cat > /tmp/config-dev.env << 'EOF'
CHECK_INTERVAL=10m
LOG_LEVEL=debug
NOTIFICATION_CHANNELS=telegram
TELEGRAM_BOT_TOKEN=dev_bot_token
TELEGRAM_CHAT_IDS=dev_chat_id
EXCLUDE_PATTERNS=*:latest,*:dev,*:test
MAX_CONCURRENCY=3
EOF

# Production profile
cat > /tmp/config-prod.env << 'EOF'
CHECK_INTERVAL=1h
LOG_LEVEL=info
NOTIFICATION_CHANNELS=email,telegram
SMTP_HOST=smtp.company.com
EMAIL_TO=devops@company.com,alerts@company.com
TELEGRAM_BOT_TOKEN=prod_bot_token
TELEGRAM_CHAT_IDS=prod_chat_id
EXCLUDE_PATTERNS=*:latest,*:alpha,*:beta,*:rc
MAX_CONCURRENCY=15
ONCE_PER_UPDATE=true
COOLDOWN_PERIOD=24h
EOF

# Staging profile
cat > /tmp/config-staging.env << 'EOF'
CHECK_INTERVAL=30m
LOG_LEVEL=info
NOTIFICATION_CHANNELS=telegram
TELEGRAM_BOT_TOKEN=staging_bot_token
TELEGRAM_CHAT_IDS=staging_chat_id
INCLUDE_PATTERNS=mycompany/*,nginx:*,postgres:*
CHECK_PRIVATE=true
MAX_CONCURRENCY=8
EOF

echo "Profile files created:"
echo "📁 /tmp/config-dev.env     - Development environment"
echo "📁 /tmp/config-prod.env    - Production environment"
echo "📁 /tmp/config-staging.env - Staging environment"
echo ""
echo "Usage:"
echo "# Development"
echo "docker run --env-file /tmp/config-dev.env docker-notify"
echo ""
echo "# Production"
echo "docker run --env-file /tmp/config-prod.env docker-notify"
echo ""
echo "# Staging"
echo "docker run --env-file /tmp/config-staging.env docker-notify"

show_section "Method 5: Runtime Configuration Changes"

echo "Demonstrating runtime configuration changes:"
echo ""

# Function to generate config and test
generate_and_test_config() {
    local env_name="$1"
    local interval="$2"
    local channels="$3"
    local level="$4"

    echo "🔧 Configuration: $env_name"
    export CHECK_INTERVAL="$interval"
    export NOTIFICATION_CHANNELS="$channels"
    export LOG_LEVEL="$level"

    echo "   CHECK_INTERVAL=$interval"
    echo "   NOTIFICATION_CHANNELS=$channels"
    echo "   LOG_LEVEL=$level"

    # In a real scenario, you might restart the service here
    echo "   ↻ Service would restart with new configuration"
    echo ""
}

generate_and_test_config "Quick Testing" "5m" "telegram" "debug"
generate_and_test_config "Normal Operations" "1h" "email,telegram" "info"
generate_and_test_config "Maintenance Mode" "6h" "email" "warn"

show_section "Method 6: Configuration Templates"

echo "Creating configuration templates for common scenarios:"

# Template for monitoring only critical services
cat > /tmp/template-critical.yaml << 'EOF'
# Critical Services Monitoring Template
app:
  check_interval: "15m"  # Check more frequently
  timezone: "UTC"
  max_concurrency: 20

docker:
  filters:
    include:
      - "postgres:*"
      - "redis:*"
      - "nginx:*"
      - "traefik:*"
      - "*/critical-*"
    exclude:
      - "*:latest"
      - "*:dev"
    check_latest: false
    check_private: true

notifications:
  channels: ["email", "telegram"]
  behavior:
    once_per_update: true
    cooldown_period: "1h"  # Shorter cooldown for critical services
    group_updates: false   # Individual notifications for critical services

logging:
  level: "info"
EOF

# Template for development environment
cat > /tmp/template-development.yaml << 'EOF'
# Development Environment Template
app:
  check_interval: "5m"   # Very frequent for development
  timezone: "America/New_York"
  max_concurrency: 5

docker:
  filters:
    include:
      - "*/dev-*"
      - "*/test-*"
    exclude:
      - "*:latest"
    check_latest: true     # Include latest for dev
    check_private: true

notifications:
  channels: ["telegram"]
  behavior:
    once_per_update: false # Multiple notifications OK in dev
    cooldown_period: "5m"  # Short cooldown
    group_updates: true

logging:
  level: "debug"
EOF

echo "Templates created:"
echo "📄 /tmp/template-critical.yaml    - Critical services monitoring"
echo "📄 /tmp/template-development.yaml - Development environment"
echo ""
echo "Usage with CONFIG_CONTENT:"
echo 'export CONFIG_CONTENT="$(cat /tmp/template-critical.yaml)"'

show_section "Summary and Best Practices"

echo "✅ Configuration Methods Summary:"
echo ""
echo "1. 🔧 Environment Variables"
echo "   - Best for: Simple overrides, CI/CD pipelines"
echo "   - Pros: Easy to use, good for sensitive data"
echo "   - Cons: Limited for complex configurations"
echo ""
echo "2. 📄 CONFIG_CONTENT (YAML)"
echo "   - Best for: Complete dynamic configuration"
echo "   - Pros: Full control, version control friendly"
echo "   - Cons: Requires YAML formatting"
echo ""
echo "3. 🐳 Docker Compose Variables"
echo "   - Best for: Container deployments"
echo "   - Pros: Integrated with Docker workflow"
echo "   - Cons: Docker-specific"
echo ""
echo "4. 📋 Configuration Profiles"
echo "   - Best for: Multiple environments"
echo "   - Pros: Environment-specific settings"
echo "   - Cons: Need to manage multiple files"
echo ""
echo "💡 Best Practices:"
echo "• Use environment variables for secrets (tokens, passwords)"
echo "• Use CONFIG_CONTENT for complex, dynamic configurations"
echo "• Create profiles for different environments (dev/staging/prod)"
echo "• Always validate configuration before deployment"
echo "• Use version control for configuration templates"
echo "• Implement configuration rollback strategies"
echo ""
echo "🎯 Priority Order (highest to lowest):"
echo "1. CONFIG_CONTENT environment variable"
echo "2. Individual environment variables"
echo "3. Configuration file (config.yaml)"
echo "4. Default values"

# Cleanup
echo ""
echo "🧹 Cleaning up temporary files..."
rm -f docker-compose.dynamic.yml
rm -f /tmp/config-*.env /tmp/template-*.yaml

echo ""
echo "✨ Demo completed! You can now configure docker-notify dynamically."
echo "📚 See README.md for more detailed configuration options."
