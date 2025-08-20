package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	// Application settings
	App AppConfig `yaml:"app"`

	// Docker settings
	Docker DockerConfig `yaml:"docker"`

	// Registry settings
	Registry RegistryConfig `yaml:"registry"`

	// Notification settings
	Notifications NotificationConfig `yaml:"notifications"`

	// Logging settings
	Logging LoggingConfig `yaml:"logging"`
}

// AppConfig contains application-level settings
type AppConfig struct {
	// Check interval for updates (e.g., "30m", "1h", "24h")
	CheckInterval string `yaml:"check_interval" default:"30m"`

	// Timezone for scheduling (e.g., "UTC", "America/New_York")
	Timezone string `yaml:"timezone" default:"UTC"`

	// Maximum concurrent registry checks
	MaxConcurrency int `yaml:"max_concurrency" default:"10"`

	// Timeout for registry API calls
	RegistryTimeout string `yaml:"registry_timeout" default:"30s"`
}

// DockerConfig contains Docker-related settings
type DockerConfig struct {
	// Docker socket path
	SocketPath string `yaml:"socket_path" default:"unix:///var/run/docker.sock"`

	// API version to use
	APIVersion string `yaml:"api_version" default:"1.43"`

	// Image filters
	Filters ImageFilters `yaml:"filters"`
}

// ImageFilters defines which images to include/exclude
type ImageFilters struct {
	// Whitelist of image patterns (if empty, all images are checked)
	Include []string `yaml:"include"`

	// Blacklist of image patterns
	Exclude []string `yaml:"exclude"`

	// Whether to check images with 'latest' tag
	CheckLatest bool `yaml:"check_latest" default:"false"`

	// Whether to check private registry images
	CheckPrivate bool `yaml:"check_private" default:"true"`

	// Version filtering options
	VersionFilters VersionFilters `yaml:"version_filters"`
}

// VersionFilters defines which version tags to exclude
type VersionFilters struct {
	// Exclude pre-release versions (alpha, beta, rc, dev, etc.)
	ExcludePreRelease bool `yaml:"exclude_prerelease" default:"true"`

	// Exclude Windows variants
	ExcludeWindows bool `yaml:"exclude_windows" default:"true"`

	// Custom patterns to exclude from version tags
	ExcludePatterns []string `yaml:"exclude_patterns"`

	// Only consider stable semantic versions (x.y.z format)
	OnlyStable bool `yaml:"only_stable" default:"true"`
}

// RegistryConfig contains registry-related settings
type RegistryConfig struct {
	// Default registry (usually DockerHub)
	DefaultRegistry string `yaml:"default_registry" default:"docker.io"`

	// Custom registries with authentication
	Registries []RegistryAuth `yaml:"registries"`

	// Rate limiting settings
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}

// RegistryAuth contains authentication info for a registry
type RegistryAuth struct {
	// Registry hostname
	Host string `yaml:"host"`

	// Username for authentication
	Username string `yaml:"username"`

	// Password for authentication
	Password string `yaml:"password"`

	// Whether to use insecure connection
	Insecure bool `yaml:"insecure" default:"false"`
}

// RateLimitConfig defines rate limiting for registry API calls
type RateLimitConfig struct {
	// Requests per minute
	RequestsPerMinute int `yaml:"requests_per_minute" default:"100"`

	// Burst limit
	Burst int `yaml:"burst" default:"10"`
}

// NotificationConfig contains all notification settings
type NotificationConfig struct {
	// Enabled notification channels
	Channels []string `yaml:"channels"`

	// Email configuration
	Email EmailConfig `yaml:"email"`

	// Telegram configuration
	Telegram TelegramConfig `yaml:"telegram"`

	// Notification templates
	Templates TemplateConfig `yaml:"templates"`

	// Notification behavior
	Behavior NotificationBehavior `yaml:"behavior"`
}

// EmailConfig contains email notification settings
type EmailConfig struct {
	// SMTP settings
	SMTP SMTPConfig `yaml:"smtp"`

	// Email addresses
	From string   `yaml:"from"`
	To   []string `yaml:"to"`

	// Email subject template
	Subject string `yaml:"subject" default:"Docker Image Updates Available"`
}

// SMTPConfig contains SMTP server settings
type SMTPConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port" default:"587"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	UseTLS   bool   `yaml:"use_tls" default:"true"`
}

// TelegramConfig contains Telegram bot settings
type TelegramConfig struct {
	// Bot token from BotFather
	BotToken string `yaml:"bot_token"`

	// Chat IDs to send messages to
	ChatIDs []int64 `yaml:"chat_ids"`

	// Whether to use HTML formatting
	ParseMode string `yaml:"parse_mode" default:"HTML"`
}

// TemplateConfig contains notification templates
type TemplateConfig struct {
	// Email templates
	EmailSubject string `yaml:"email_subject"`
	EmailBody    string `yaml:"email_body"`

	// Telegram templates
	TelegramMessage string `yaml:"telegram_message"`
}

// NotificationBehavior defines when and how to send notifications
type NotificationBehavior struct {
	// Only notify once per image update
	OncePerUpdate bool `yaml:"once_per_update" default:"true"`

	// Minimum time between notifications for the same image
	CooldownPeriod string `yaml:"cooldown_period" default:"24h"`

	// Group multiple updates into a single notification
	GroupUpdates bool `yaml:"group_updates" default:"true"`

	// Maximum number of updates to include in a single notification
	MaxUpdatesPerNotification int `yaml:"max_updates_per_notification" default:"10"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	// Log level (debug, info, warn, error)
	Level string `yaml:"level" default:"info"`

	// Log format (json, text)
	Format string `yaml:"format" default:"json"`

	// Log file path (empty for stdout)
	File string `yaml:"file"`

	// Maximum log file size in MB
	MaxSize int `yaml:"max_size" default:"100"`

	// Maximum number of old log files to retain
	MaxBackups int `yaml:"max_backups" default:"3"`

	// Maximum age of log files in days
	MaxAge int `yaml:"max_age" default:"30"`
}

// LoadConfig loads configuration from file with environment variable overrides
func LoadConfig(configPath string) (*Config, error) {
	// Set default config
	config := &Config{
		App: AppConfig{
			CheckInterval:   "30m",
			Timezone:        "UTC",
			MaxConcurrency:  10,
			RegistryTimeout: "30s",
		},
		Docker: DockerConfig{
			SocketPath: "unix:///var/run/docker.sock",
			APIVersion: "1.43",
			Filters: ImageFilters{
				CheckLatest:  false,
				CheckPrivate: true,
				VersionFilters: VersionFilters{
					ExcludePreRelease: true,
					ExcludeWindows:    true,
					OnlyStable:        true,
				},
			},
		},
		Registry: RegistryConfig{
			DefaultRegistry: "docker.io",
			RateLimit: RateLimitConfig{
				RequestsPerMinute: 100,
				Burst:             10,
			},
		},
		Notifications: NotificationConfig{
			Email: EmailConfig{
				SMTP: SMTPConfig{
					Port:   587,
					UseTLS: true,
				},
				Subject: "Docker Image Updates Available",
			},
			Telegram: TelegramConfig{
				ParseMode: "HTML",
			},
			Behavior: NotificationBehavior{
				OncePerUpdate:             true,
				CooldownPeriod:            "24h",
				GroupUpdates:              true,
				MaxUpdatesPerNotification: 10,
			},
		},
		Logging: LoggingConfig{
			Level:      "info",
			Format:     "json",
			MaxSize:    100,
			MaxBackups: 3,
			MaxAge:     30,
		},
	}

	// Load from file if it exists
	if configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			data, err := os.ReadFile(configPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read config file: %w", err)
			}

			if err := yaml.Unmarshal(data, config); err != nil {
				return nil, fmt.Errorf("failed to parse config file: %w", err)
			}
		}
	}

	// Override with environment variables
	if err := config.loadFromEnv(); err != nil {
		return nil, fmt.Errorf("failed to load environment variables: %w", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// loadFromEnv loads configuration from environment variables
func (c *Config) loadFromEnv() error {
	// App config
	if val := os.Getenv("CHECK_INTERVAL"); val != "" {
		c.App.CheckInterval = val
	}
	if val := os.Getenv("TIMEZONE"); val != "" {
		c.App.Timezone = val
	}

	// Docker config
	if val := os.Getenv("DOCKER_SOCKET"); val != "" {
		c.Docker.SocketPath = val
	}
	if val := os.Getenv("DOCKER_API_VERSION"); val != "" {
		c.Docker.APIVersion = val
	}

	// Notification config
	if val := os.Getenv("SMTP_HOST"); val != "" {
		c.Notifications.Email.SMTP.Host = val
	}
	if val := os.Getenv("SMTP_USERNAME"); val != "" {
		c.Notifications.Email.SMTP.Username = val
	}
	if val := os.Getenv("SMTP_PASSWORD"); val != "" {
		c.Notifications.Email.SMTP.Password = val
	}
	if val := os.Getenv("EMAIL_FROM"); val != "" {
		c.Notifications.Email.From = val
	}
	if val := os.Getenv("EMAIL_TO"); val != "" {
		c.Notifications.Email.To = []string{val}
	}
	if val := os.Getenv("TELEGRAM_BOT_TOKEN"); val != "" {
		c.Notifications.Telegram.BotToken = val
	}

	return nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate check interval
	if _, err := time.ParseDuration(c.App.CheckInterval); err != nil {
		return fmt.Errorf("invalid check_interval: %w", err)
	}

	// Validate registry timeout
	if _, err := time.ParseDuration(c.App.RegistryTimeout); err != nil {
		return fmt.Errorf("invalid registry_timeout: %w", err)
	}

	// Validate cooldown period
	if _, err := time.ParseDuration(c.Notifications.Behavior.CooldownPeriod); err != nil {
		return fmt.Errorf("invalid cooldown_period: %w", err)
	}

	// Validate notification channels
	for _, channel := range c.Notifications.Channels {
		switch channel {
		case "email":
			if c.Notifications.Email.SMTP.Host == "" {
				return fmt.Errorf("email channel enabled but SMTP host not configured")
			}
			if len(c.Notifications.Email.To) == 0 {
				return fmt.Errorf("email channel enabled but no recipients configured")
			}
		case "telegram":
			if c.Notifications.Telegram.BotToken == "" {
				return fmt.Errorf("telegram channel enabled but bot token not configured")
			}
			if len(c.Notifications.Telegram.ChatIDs) == 0 {
				return fmt.Errorf("telegram channel enabled but no chat IDs configured")
			}
		default:
			return fmt.Errorf("unknown notification channel: %s", channel)
		}
	}

	return nil
}

// GetCheckInterval returns the check interval as a time.Duration
func (c *Config) GetCheckInterval() time.Duration {
	duration, _ := time.ParseDuration(c.App.CheckInterval)
	return duration
}

// GetRegistryTimeout returns the registry timeout as a time.Duration
func (c *Config) GetRegistryTimeout() time.Duration {
	duration, _ := time.ParseDuration(c.App.RegistryTimeout)
	return duration
}

// GetCooldownPeriod returns the cooldown period as a time.Duration
func (c *Config) GetCooldownPeriod() time.Duration {
	duration, _ := time.ParseDuration(c.Notifications.Behavior.CooldownPeriod)
	return duration
}

// IsNotificationChannelEnabled checks if a notification channel is enabled
func (c *Config) IsNotificationChannelEnabled(channel string) bool {
	for _, ch := range c.Notifications.Channels {
		if ch == channel {
			return true
		}
	}
	return false
}
