# Coolify Deployment Guide for Docker Notify

This guide provides step-by-step instructions for deploying docker-notify on Coolify, including solutions for common deployment issues.

## Overview

Docker Notify is a service that monitors Docker containers for image updates and sends notifications via Telegram, email, or other channels. This guide covers the specific considerations for deploying it on Coolify.

## Quick Start

1. **Fork or clone this repository** to your Git provider (GitHub, GitLab, etc.)

2. **Create a new application** in Coolify:
   - Choose "Docker Compose" as the deployment type
   - Connect your Git repository
   - Set the branch (usually `main` or `master`)

3. **Configure environment variables** (see Configuration section below)

4. **Deploy** the application

## Architecture Considerations

### Multi-Architecture Support

The Dockerfile has been updated to support multiple architectures:
- `linux/amd64` (Intel/AMD 64-bit)
- `linux/arm64` (ARM 64-bit, including Apple Silicon)
- `linux/arm/v7` (ARM 32-bit)

If you encounter `exec format error`, your server architecture may not be supported. Check your server architecture:

```bash
uname -m
```

## Configuration

### Method 1: Environment Variables (Recommended for Coolify)

Set these environment variables in your Coolify application:

#### Required Variables

```bash
# Telegram Configuration (if using Telegram notifications)
TELEGRAM_BOT_TOKEN=your_bot_token_here
TELEGRAM_CHAT_IDS=123456789,-987654321

# OR Email Configuration (if using email notifications)
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=your-email@gmail.com
SMTP_PASSWORD=your-app-password
EMAIL_FROM=docker-notify@yourdomain.com
EMAIL_TO=admin@yourdomain.com

# Notification channels to enable
NOTIFICATION_CHANNELS=telegram
# or
NOTIFICATION_CHANNELS=email
# or
NOTIFICATION_CHANNELS=telegram,email
```

#### Optional Variables

```bash
# Check interval (how often to check for updates)
CHECK_INTERVAL=30m

# Timezone
TIMEZONE=UTC

# Docker filters
EXCLUDE_PATTERNS=*:latest,scratch:*
CHECK_LATEST=false
CHECK_PRIVATE=true
EXCLUDE_PRERELEASE=true

# Notification behavior
ONCE_PER_UPDATE=true
COOLDOWN_PERIOD=24h
GROUP_UPDATES=true
MAX_UPDATES_PER_NOTIFICATION=10

# Logging
LOG_LEVEL=info
```

### Method 2: Configuration File (Alternative)

If you prefer using a configuration file, you can provide the entire configuration as a base64-encoded environment variable:

1. **Generate the configuration**:
   ```bash
   # In your local repository
   ./scripts/generate-config-env.sh
   ```

2. **Set the environment variable** in Coolify:
   ```bash
   CONFIG_CONTENT_BASE64=<base64-encoded-config>
   ```

## Docker Socket Access

Docker Notify needs access to the Docker daemon to monitor containers. This is handled automatically by mounting `/var/run/docker.sock`.

### Security Note

The container runs as root (user 0) to access the Docker socket. This is necessary for Docker API access but grants significant privileges. In production environments, consider:

1. Running on a dedicated Docker host
2. Using Docker-in-Docker with proper isolation
3. Implementing additional security measures as per your organization's policies

## Troubleshooting

### Issue 1: "exec format error"

**Problem**: Container fails to start with architecture mismatch error.

**Solution**: 
1. Check your server architecture: `uname -m`
2. If using ARM64, ensure you're pulling the correct image
3. For custom builds, use the multi-architecture build script:
   ```bash
   ./scripts/build-multiarch.sh --platforms linux/arm64
   ```

### Issue 2: "permission denied while trying to connect to Docker daemon"

**Problem**: Container can't access Docker socket.

**Solutions**:
1. Ensure the docker-compose.yml includes `user: "0:0"` (already configured)
2. Verify Docker socket is accessible on the host
3. Check Coolify's Docker socket mounting configuration

### Issue 3: "failed to open log file"

**Problem**: Container tries to write to a log file that doesn't exist.

**Solution**: This is fixed in the default configuration (logs to stdout). If you need file logging, ensure the log directory is properly mounted.

### Issue 4: Container exits with code 1

**Problem**: Container starts but immediately exits.

**Debugging steps**:
1. Check the container logs in Coolify
2. Verify all required environment variables are set
3. Ensure Docker socket is accessible
4. Check the health check configuration

### Issue 5: No notifications received

**Problem**: Service runs but doesn't send notifications.

**Debugging**:
1. Set `LOG_LEVEL=debug` to see more details
2. Verify notification credentials (bot token, email settings)
3. Check that containers actually have updates available
4. Verify notification channels are properly configured

## Environment Variables Reference

### Application Settings
- `CHECK_INTERVAL`: How often to check for updates (default: "30m")
- `TIMEZONE`: Timezone for scheduling (default: "UTC")
- `MAX_CONCURRENCY`: Max concurrent registry calls (default: 10)
- `REGISTRY_TIMEOUT`: Timeout for registry calls (default: "30s")

### Docker Configuration
- `DOCKER_SOCKET`: Docker socket path (default: "unix:///var/run/docker.sock")
- `DOCKER_API_VERSION`: Docker API version (auto-negotiated if empty)

### Filtering
- `INCLUDE_PATTERNS`: Only check these patterns (empty = all)
- `EXCLUDE_PATTERNS`: Exclude these patterns (default: "*:latest,scratch:*")
- `CHECK_LATEST`: Check latest tags (default: false)
- `CHECK_PRIVATE`: Check private registries (default: true)
- `EXCLUDE_PRERELEASE`: Exclude pre-release versions (default: true)
- `EXCLUDE_WINDOWS`: Exclude Windows variants (default: true)
- `ONLY_STABLE`: Only stable semantic versions (default: true)

### Notifications
- `NOTIFICATION_CHANNELS`: Enabled channels (e.g., "telegram,email")
- `ONCE_PER_UPDATE`: Notify once per update (default: true)
- `COOLDOWN_PERIOD`: Min time between notifications (default: "24h")
- `GROUP_UPDATES`: Group multiple updates (default: true)
- `MAX_UPDATES_PER_NOTIFICATION`: Max updates per notification (default: 10)

### Telegram
- `TELEGRAM_BOT_TOKEN`: Bot token from @BotFather
- `TELEGRAM_CHAT_IDS`: Comma-separated chat IDs
- `TELEGRAM_PARSE_MODE`: Message format (HTML, Markdown, or empty)

### Email
- `SMTP_HOST`: SMTP server hostname
- `SMTP_PORT`: SMTP server port (default: 587)
- `SMTP_USERNAME`: SMTP username
- `SMTP_PASSWORD`: SMTP password
- `SMTP_USE_TLS`: Use TLS encryption (default: true)
- `EMAIL_FROM`: From email address
- `EMAIL_TO`: To email address(es)
- `EMAIL_SUBJECT`: Email subject prefix

### Logging
- `LOG_LEVEL`: Log level (debug, info, warn, error)

## Health Checks

The application includes health checks that verify:
1. Application is running
2. Configuration is valid
3. Docker daemon is accessible

Health check endpoint is available internally for monitoring.

## Resource Usage

Default resource limits:
- Memory: 128MB limit, 64MB reservation
- CPU: 0.5 cores limit, 0.1 cores reservation

These can be adjusted in the docker-compose.yml if needed.

## Security Best Practices

1. **Use app-specific passwords** for email (not your main password)
2. **Secure your bot tokens** - store them as secrets in Coolify
3. **Monitor access logs** for any unusual activity
4. **Keep the image updated** to get security patches
5. **Use private registries** when possible for sensitive applications
6. **Limit notification recipients** to authorized personnel only

## Support

For issues specific to this deployment:
1. Check the troubleshooting section above
2. Review Coolify logs for deployment issues
3. Check the application logs for runtime issues
4. Ensure all environment variables are correctly set

For general docker-notify issues, refer to the main README.md.