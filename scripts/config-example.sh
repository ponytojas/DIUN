#!/bin/bash

# Docker Notify Configuration Example
# This script demonstrates how to configure docker-notify using environment variables
# You can source this file or copy these variables to your .env file

# =============================================================================
# APPLICATION SETTINGS
# =============================================================================

# How often to check for image updates (examples: "30m", "1h", "24h")
export CHECK_INTERVAL="30m"

# Timezone for scheduling (examples: "UTC", "America/New_York", "Europe/London")
export TIMEZONE="UTC"

# Maximum number of concurrent registry API calls
export MAX_CONCURRENCY="10"

# Timeout for registry API calls
export REGISTRY_TIMEOUT="30s"

# =============================================================================
# DOCKER SETTINGS
# =============================================================================

# Docker socket path (usually unix:///var/run/docker.sock)
export DOCKER_SOCKET="unix:///var/run/docker.sock"

# Docker API version (leave empty for auto-negotiation)
export DOCKER_API_VERSION=""

# =============================================================================
# IMAGE FILTERING
# =============================================================================

# Whether to check images with 'latest' tag (can be unreliable)
export CHECK_LATEST="false"

# Whether to check images from private registries
export CHECK_PRIVATE="true"

# Whitelist: only check these image patterns (comma-separated, empty = check all)
# Examples: "nginx:*,postgres:*,myregistry.com/*"
export INCLUDE_PATTERNS=""

# Blacklist: exclude these image patterns (comma-separated)
# Examples: "*:latest,scratch:*,alpine:*"
export EXCLUDE_PATTERNS="*:latest,scratch:*"

# Exclude pre-release versions (alpha, beta, rc, dev, etc.)
export EXCLUDE_PRERELEASE="true"

# Exclude Windows variants (windowsservercore, nanoserver, etc.)
export EXCLUDE_WINDOWS="true"

# Only consider stable semantic versions (x.y.z format)
export ONLY_STABLE="true"

# =============================================================================
# NOTIFICATION SETTINGS
# =============================================================================

# Enabled notification channels (comma-separated): "email,telegram"
export NOTIFICATION_CHANNELS="telegram"

# =============================================================================
# EMAIL NOTIFICATION SETTINGS
# =============================================================================

# SMTP server configuration
export SMTP_HOST="smtp.gmail.com"
export SMTP_PORT="587"
export SMTP_USERNAME="your-email@gmail.com"
export SMTP_PASSWORD="your-app-password"
export SMTP_USE_TLS="true"

# Email addresses
export EMAIL_FROM="docker-notify@yourdomain.com"
export EMAIL_TO="admin@yourdomain.com,devops@yourdomain.com"

# Email subject
export EMAIL_SUBJECT="Docker Image Updates Available"

# =============================================================================
# TELEGRAM NOTIFICATION SETTINGS
# =============================================================================

# Bot token from @BotFather
export TELEGRAM_BOT_TOKEN="YOUR_BOT_TOKEN_HERE"

# Chat IDs to send messages to (comma-separated)
# You can get your chat ID by messaging @userinfobot
# Use negative numbers for group chats
export TELEGRAM_CHAT_IDS="123456789,-987654321"

# Message formatting (HTML, Markdown, or empty for plain text)
export TELEGRAM_PARSE_MODE="HTML"

# =============================================================================
# NOTIFICATION BEHAVIOR
# =============================================================================

# Only notify once per image update (avoid spam)
export ONCE_PER_UPDATE="true"

# Minimum time between notifications for the same image
export COOLDOWN_PERIOD="24h"

# Group multiple updates into a single notification
export GROUP_UPDATES="true"

# Maximum number of updates to include in a single notification
export MAX_UPDATES_PER_NOTIFICATION="10"

# =============================================================================
# LOGGING SETTINGS
# =============================================================================

# Log level: debug, info, warn, error
export LOG_LEVEL="info"

# =============================================================================
# ALTERNATIVE: PASS ENTIRE CONFIG AS YAML
# =============================================================================

# Instead of using individual environment variables, you can pass the entire
# configuration as YAML content. This takes precedence over the config file.
# Uncomment and modify the following to use this approach:

# export CONFIG_CONTENT='
# app:
#   check_interval: "30m"
#   timezone: "UTC"
#   max_concurrency: 10
#   registry_timeout: "30s"
#
# docker:
#   socket_path: "unix:///var/run/docker.sock"
#   api_version: ""
#   filters:
#     include: []
#     exclude:
#       - "*:latest"
#       - "scratch:*"
#     check_latest: false
#     check_private: true
#     version_filters:
#       exclude_prerelease: true
#       exclude_windows: true
#       only_stable: true
#
# notifications:
#   channels:
#     - "telegram"
#   telegram:
#     bot_token: "YOUR_BOT_TOKEN_HERE"
#     chat_ids:
#       - 123456789
#     parse_mode: "HTML"
#   behavior:
#     once_per_update: true
#     cooldown_period: "24h"
#     group_updates: true
#     max_updates_per_notification: 10
#
# logging:
#   level: "info"
#   format: "json"
# '

# =============================================================================
# USAGE EXAMPLES
# =============================================================================

echo "Docker Notify Configuration Example"
echo "===================================="
echo ""
echo "To use these environment variables:"
echo ""
echo "1. Copy this file and modify the values:"
echo "   cp scripts/config-example.sh my-config.sh"
echo "   # Edit my-config.sh with your values"
echo ""
echo "2. Source the configuration before running docker-compose:"
echo "   source my-config.sh"
echo "   docker-compose up -d"
echo ""
echo "3. Or create a .env file with these variables:"
echo "   cp scripts/config-example.sh .env"
echo "   # Edit .env file (remove 'export ' prefixes)"
echo "   docker-compose up -d"
echo ""
echo "4. Or pass them directly to docker-compose:"
echo "   CHECK_INTERVAL=1h TELEGRAM_BOT_TOKEN=your_token docker-compose up -d"
echo ""
echo "Current configuration would be:"
echo "- Check interval: $CHECK_INTERVAL"
echo "- Timezone: $TIMEZONE"
echo "- Notification channels: $NOTIFICATION_CHANNELS"
echo "- Log level: $LOG_LEVEL"
