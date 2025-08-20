# Docker Notify - Container Image Update Notification Service

[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org)
[![Docker](https://img.shields.io/badge/Docker-Ready-blue.svg)](https://docker.com)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

A lightweight, efficient background service that monitors Docker containers and notifies you when newer image versions are available. Designed for deployment in **Coolify** and other containerized environments.

## 🌟 Features

- **Automatic Detection**: Monitors all running containers on the host
- **Multiple Registries**: Supports DockerHub and private registries
- **Smart Filtering**: Configurable include/exclude patterns for images
- **Multiple Notification Channels**: Email (SMTP) and Telegram Bot support
- **Semantic Versioning**: Intelligent version comparison for updates
- **Rate Limiting**: Respects registry API limits
- **Scheduling**: Configurable check intervals with cron-like scheduling
- **Production Ready**: Comprehensive logging, health checks, and error handling
- **Resource Efficient**: Minimal CPU and memory footprint
- **Containerized**: Ready for deployment in Docker/Kubernetes/Coolify

## 🏗️ Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Scheduler     │───▶│  Docker Scanner  │───▶│ Registry Client │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                │                        │
                                ▼                        ▼
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│ Notification    │◀───│   Controller     │◀───│  Version Comp.  │
│    System       │    └──────────────────┘    └─────────────────┘
└─────────────────┘
```

The service follows a modular architecture:
- **Docker Scanner**: Detects running containers and extracts image information
- **Registry Client**: Queries DockerHub/registries for available updates
- **Version Comparator**: Intelligently compares semantic versions
- **Notification System**: Sends alerts via multiple channels
- **Scheduler**: Manages periodic checks and task execution

## 🚀 Quick Start

### Option 1: Docker Compose (Recommended)

1. **Clone the repository**:
```bash
git clone <repository-url>
cd docker-notify
```

2. **Configure the service**:
```bash
cp configs/config.yaml configs/config.local.yaml
# Edit configs/config.local.yaml with your settings
```

3. **Set up environment variables**:
```bash
# Create .env file
cat > .env << EOF
SMTP_HOST=smtp.gmail.com
SMTP_USERNAME=your-email@gmail.com
SMTP_PASSWORD=your-app-password
EMAIL_FROM=docker-notify@yourdomain.com
EMAIL_TO=admin@yourdomain.com
TELEGRAM_BOT_TOKEN=YOUR_BOT_TOKEN_HERE
EOF
```

4. **Run with Docker Compose**:
```bash
docker-compose up -d
```

### Option 2: Direct Docker Run

```bash
# Build the image
docker build -t docker-notify .

# Run the container
docker run -d \
  --name docker-notify \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -v ./configs/config.yaml:/etc/docker-notify/config.yaml:ro \
  -e SMTP_HOST=smtp.gmail.com \
  -e SMTP_USERNAME=your-email@gmail.com \
  -e SMTP_PASSWORD=your-app-password \
  -e EMAIL_TO=admin@yourdomain.com \
  docker-notify
```

### Option 3: Binary Installation

```bash
# Build from source
go mod download
go build -o docker-notify ./cmd/main.go

# Run directly
./docker-notify -config configs/config.yaml
```

## ⚙️ Configuration

### Configuration File

Create a `config.yaml` file with your settings:

```yaml
# Application settings
app:
  check_interval: "30m"  # Check every 30 minutes
  timezone: "UTC"
  max_concurrency: 10

# Docker settings
docker:
  socket_path: "unix:///var/run/docker.sock"
  filters:
    exclude:
      - "*:latest"      # Skip latest tags
      - "scratch:*"     # Skip scratch images
    check_latest: false
    check_private: true

# Notification settings
notifications:
  channels:
    - "email"
    - "telegram"
  
  email:
    smtp:
      host: "smtp.gmail.com"
      port: 587
      username: "your-email@gmail.com"
      password: "your-app-password"
      use_tls: true
    from: "docker-notify@yourdomain.com"
    to:
      - "admin@yourdomain.com"
  
  telegram:
    bot_token: "YOUR_BOT_TOKEN_HERE"
    chat_ids:
      - 123456789
    parse_mode: "HTML"

# Logging
logging:
  level: "info"
  format: "json"
```

### Environment Variables

You can override configuration with environment variables:

| Variable | Description | Example |
|----------|-------------|---------|
| `CHECK_INTERVAL` | How often to check for updates | `30m`, `1h`, `24h` |
| `DOCKER_SOCKET` | Docker socket path | `unix:///var/run/docker.sock` |
| `SMTP_HOST` | SMTP server hostname | `smtp.gmail.com` |
| `SMTP_USERNAME` | SMTP username | `your-email@gmail.com` |
| `SMTP_PASSWORD` | SMTP password | `your-app-password` |
| `EMAIL_FROM` | From email address | `docker-notify@yourdomain.com` |
| `EMAIL_TO` | To email address | `admin@yourdomain.com` |
| `TELEGRAM_BOT_TOKEN` | Telegram bot token | `123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11` |
| `LOG_LEVEL` | Log level | `debug`, `info`, `warn`, `error` |

## 📧 Notification Setup

### Email (SMTP)

1. **Gmail Setup**:
   - Enable 2-factor authentication
   - Generate an App Password
   - Use `smtp.gmail.com:587` with TLS

2. **Outlook Setup**:
   - Use `smtp-mail.outlook.com:587` with TLS

3. **Custom SMTP**:
   - Configure your SMTP server details

### Telegram Bot

1. **Create a Bot**:
   - Message [@BotFather](https://t.me/botfather) on Telegram
   - Send `/newbot` and follow instructions
   - Get your bot token

2. **Get Chat ID**:
   - Message [@userinfobot](https://t.me/userinfobot) to get your chat ID
   - For group chats, add the bot to the group and use negative chat ID

3. **Test the Bot**:
   ```bash
   curl -X POST "https://api.telegram.org/bot<YOUR_BOT_TOKEN>/sendMessage" \
        -H "Content-Type: application/json" \
        -d '{"chat_id": "<YOUR_CHAT_ID>", "text": "Test message"}'
   ```

## 🚀 Deployment

### Coolify Deployment

1. **Create a new service** in Coolify
2. **Set the Docker image**: Use the built image or build from source
3. **Configure environment variables** in the Coolify dashboard
4. **Mount volumes**:
   - `/var/run/docker.sock:/var/run/docker.sock:ro`
   - `./config.yaml:/etc/docker-notify/config.yaml:ro`
5. **Deploy** the service

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: docker-notify
spec:
  replicas: 1
  selector:
    matchLabels:
      app: docker-notify
  template:
    metadata:
      labels:
        app: docker-notify
    spec:
      containers:
      - name: docker-notify
        image: docker-notify:latest
        env:
        - name: SMTP_HOST
          value: "smtp.gmail.com"
        - name: EMAIL_TO
          value: "admin@yourdomain.com"
        volumeMounts:
        - name: docker-socket
          mountPath: /var/run/docker.sock
          readOnly: true
        - name: config
          mountPath: /etc/docker-notify/config.yaml
          subPath: config.yaml
      volumes:
      - name: docker-socket
        hostPath:
          path: /var/run/docker.sock
      - name: config
        configMap:
          name: docker-notify-config
```

### Docker Swarm

```yaml
version: '3.8'
services:
  docker-notify:
    image: docker-notify:latest
    deploy:
      replicas: 1
      placement:
        constraints:
          - node.role == manager
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    environment:
      - SMTP_HOST=smtp.gmail.com
      - EMAIL_TO=admin@yourdomain.com
    networks:
      - docker-notify-net
```

## 🔧 Command Line Usage

```bash
# Run with default config
./docker-notify

# Specify config file
./docker-notify -config /path/to/config.yaml

# Run a single check and exit
./docker-notify -check-once

# Test notifications and exit
./docker-notify -test

# Set log level
./docker-notify -log-level debug

# Show version
./docker-notify -version
```

## 📊 Monitoring & Logging

### Health Checks

The service includes built-in health checks:

```bash
# Docker health check
docker exec docker-notify /docker-notify -test

# HTTP health endpoint (if enabled)
curl http://localhost:8080/health
```

### Logs

Logs are structured in JSON format:

```json
{
  "level": "info",
  "msg": "Found 3 image updates",
  "time": "2024-01-01T12:00:00Z",
  "updates_found": 3,
  "checked_count": 15,
  "duration": "2.5s"
}
```

### Metrics

Key metrics tracked:
- Number of containers checked
- Updates found and notified
- Registry API call success rate
- Notification delivery status
- Task execution times

## 🔍 Troubleshooting

### Common Issues

1. **Docker Socket Permission Denied**:
   ```bash
   # Add user to docker group
   sudo usermod -aG docker $USER
   
   # Or run with appropriate permissions
   sudo ./docker-notify
   ```

2. **Registry Rate Limits**:
   - Reduce `check_interval` frequency
   - Adjust `rate_limit` settings in config
   - Use registry authentication

3. **Email Not Sending**:
   - Check SMTP credentials
   - Verify firewall/network settings
   - Test with telnet: `telnet smtp.gmail.com 587`

4. **Telegram Bot Not Working**:
   - Verify bot token
   - Check chat ID (use @userinfobot)
   - Ensure bot can send messages to chat

### Debug Mode

Run with debug logging:

```bash
./docker-notify -log-level debug
```

### Test Configuration

```bash
# Test all components
./docker-notify -test

# Test specific notification channel
curl -X POST localhost:8080/api/test/email
curl -X POST localhost:8080/api/test/telegram
```

## 🛡️ Security Considerations

### Permissions

- **Docker Socket**: Read-only access to `/var/run/docker.sock`
- **User**: Run as non-root user (appuser) in container
- **Network**: Only outbound connections to registries and notification services

### Secrets Management

- Use environment variables for sensitive data
- Consider external secret management (Vault, K8s secrets)
- Rotate credentials regularly

### Network Security

- Run in isolated network
- Use TLS for all external communications
- Consider using registry mirrors/proxies

## 🚀 Future Enhancements

### Planned Features

- [ ] **Web Dashboard**: Real-time monitoring interface
- [ ] **More Notification Channels**: Slack, Discord, Webhook support
- [ ] **GitOps Integration**: Automatic PR creation for updates
- [ ] **Custom Update Policies**: Skip patch versions, pre-release handling
- [ ] **Multi-Host Support**: Monitor multiple Docker hosts
- [ ] **Metrics Export**: Prometheus metrics endpoint
- [ ] **Update Automation**: Automatic image updates with rollback
- [ ] **Security Scanning**: Vulnerability alerts for images

### API Endpoints

Future REST API for:
- Manual trigger checks
- Configuration management
- Status and metrics
- Notification testing

## 🤝 Contributing

1. **Fork** the repository
2. **Create** a feature branch: `git checkout -b feature/amazing-feature`
3. **Commit** your changes: `git commit -m 'Add amazing feature'`
4. **Push** to the branch: `git push origin feature/amazing-feature`
5. **Open** a Pull Request

### Development Setup

```bash
# Clone repository
git clone <repository-url>
cd docker-notify

# Install dependencies
go mod download

# Run tests
go test ./...

# Build
go build -o docker-notify ./cmd/main.go

# Run locally
./docker-notify -config configs/config.yaml
```

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Docker API for container introspection
- DockerHub Registry API for image metadata
- Go community for excellent libraries:
  - [logrus](https://github.com/sirupsen/logrus) for logging
  - [robfig/cron](https://github.com/robfig/cron) for scheduling
  - [go-telegram-bot-api](https://github.com/go-telegram-bot-api/telegram-bot-api) for Telegram
  - [gomail](https://gopkg.in/gomail.v2) for email notifications

## 📞 Support

- **Issues**: [GitHub Issues](https://github.com/your-username/docker-notify/issues)
- **Discussions**: [GitHub Discussions](https://github.com/your-username/docker-notify/discussions)
- **Documentation**: [Wiki](https://github.com/your-username/docker-notify/wiki)

---

**Made with ❤️ for the Docker community**