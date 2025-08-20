package main

import (
	"context"
	"docker-notify/internal/config"
	"docker-notify/internal/docker"
	"docker-notify/internal/notifications"
	"docker-notify/internal/registry"
	"docker-notify/internal/scheduler"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	appName    = "docker-notify"
	appVersion = "1.0.0"
)

// Service represents the main application service
type Service struct {
	config        *config.Config
	logger        *logrus.Logger
	dockerClient  *docker.Client
	registry      *registry.Client
	notifications *notifications.Manager
	scheduler     *scheduler.Scheduler
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

func main() {
	// Parse command line flags
	var (
		configPath = flag.String("config", "/etc/docker-notify/config.yaml", "Path to configuration file")
		logLevel   = flag.String("log-level", "", "Log level (debug, info, warn, error)")
		version    = flag.Bool("version", false, "Show version information")
		testMode   = flag.Bool("test", false, "Run in test mode (send test notifications and exit)")
		checkOnce  = flag.Bool("check-once", false, "Run image check once and exit")
	)
	flag.Parse()

	// Show version and exit
	if *version {
		fmt.Printf("%s version %s\n", appName, appVersion)
		os.Exit(0)
	}

	// Create logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		logger.WithError(err).Fatal("Failed to load configuration")
	}

	// Override log level from command line
	if *logLevel != "" {
		cfg.Logging.Level = *logLevel
	}

	// Configure logger
	if err := configureLogger(logger, cfg.Logging); err != nil {
		logger.WithError(err).Fatal("Failed to configure logger")
	}

	logger.WithFields(logrus.Fields{
		"version":     appVersion,
		"config_path": *configPath,
	}).Info("Starting Docker Notify service")

	// Create main service
	service, err := NewService(cfg, logger)
	if err != nil {
		logger.WithError(err).Fatal("Failed to create service")
	}
	defer service.Close()

	// Handle different run modes
	switch {
	case *testMode:
		if err := service.RunTestMode(); err != nil {
			logger.WithError(err).Fatal("Test mode failed")
		}
		logger.Info("Test mode completed successfully")
		return

	case *checkOnce:
		if err := service.RunCheckOnce(); err != nil {
			logger.WithError(err).Fatal("Single check failed")
		}
		logger.Info("Single check completed successfully")
		return

	default:
		// Run in service mode
		if err := service.Run(); err != nil {
			logger.WithError(err).Fatal("Service failed")
		}
	}
}

// NewService creates a new service instance
func NewService(cfg *config.Config, logger *logrus.Logger) (*Service, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create Docker client
	dockerClient, err := docker.NewClient(cfg.Docker.SocketPath, cfg.Docker.APIVersion, logger)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	// Test Docker connection
	if err := dockerClient.Health(ctx); err != nil {
		cancel()
		return nil, fmt.Errorf("Docker daemon health check failed: %w", err)
	}

	// Create registry client with version filters
	versionFilters := registry.VersionFilterConfig{
		ExcludePreRelease: cfg.Docker.Filters.VersionFilters.ExcludePreRelease,
		ExcludeWindows:    cfg.Docker.Filters.VersionFilters.ExcludeWindows,
		ExcludePatterns:   cfg.Docker.Filters.VersionFilters.ExcludePatterns,
		OnlyStable:        cfg.Docker.Filters.VersionFilters.OnlyStable,
	}

	registryClient := registry.NewClientWithFilters(
		cfg.Registry.RateLimit.RequestsPerMinute,
		cfg.Registry.RateLimit.Burst,
		logger,
		versionFilters,
	)

	// Test registry connection
	if err := registryClient.Health(ctx); err != nil {
		logger.WithError(err).Warn("Registry health check failed, continuing anyway")
	}

	// Create notification manager
	notificationManager := notifications.NewManager(logger)

	// Set up notification channels
	if err := setupNotificationChannels(cfg, notificationManager, logger); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to setup notification channels: %w", err)
	}

	// Create scheduler
	sched := scheduler.NewScheduler(logger)

	return &Service{
		config:        cfg,
		logger:        logger,
		dockerClient:  dockerClient,
		registry:      registryClient,
		notifications: notificationManager,
		scheduler:     sched,
		ctx:           ctx,
		cancel:        cancel,
	}, nil
}

// Run starts the service in daemon mode
func (s *Service) Run() error {
	s.logger.Info("Starting Docker Notify service in daemon mode")

	// Set up scheduled image checking task
	if err := s.setupScheduledTasks(); err != nil {
		return fmt.Errorf("failed to setup scheduled tasks: %w", err)
	}

	// Start scheduler
	s.scheduler.Start()

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	s.logger.Info("Docker Notify service is running")

	// Wait for shutdown signal
	<-sigChan
	s.logger.Info("Received shutdown signal, stopping service")

	// Graceful shutdown
	s.cancel()
	s.scheduler.Stop()
	s.wg.Wait()

	s.logger.Info("Service stopped successfully")
	return nil
}

// RunTestMode runs the service in test mode
func (s *Service) RunTestMode() error {
	s.logger.Info("Running in test mode")

	// Test Docker connection
	if err := s.dockerClient.Health(s.ctx); err != nil {
		return fmt.Errorf("Docker health check failed: %w", err)
	}
	s.logger.Info("✓ Docker connection test passed")

	// Test registry connection
	if err := s.registry.Health(s.ctx); err != nil {
		return fmt.Errorf("Registry health check failed: %w", err)
	}
	s.logger.Info("✓ Registry connection test passed")

	// Test notification channels
	testNotification := &notifications.Notification{
		Subject:   "Docker Notify Test",
		Message:   "This is a test notification from Docker Notify service.",
		Timestamp: time.Now(),
		Type:      notifications.NotificationTypeInfo,
		Priority:  notifications.PriorityNormal,
		Data: map[string]interface{}{
			"test": true,
		},
	}

	if err := s.notifications.Send(s.ctx, testNotification); err != nil {
		return fmt.Errorf("Failed to send test notification: %w", err)
	}
	s.logger.Info("✓ Notification test passed")

	return nil
}

// RunCheckOnce runs a single image check
func (s *Service) RunCheckOnce() error {
	s.logger.Info("Running single image check")
	return s.performImageCheck()
}

// performImageCheck performs the main image checking logic
func (s *Service) performImageCheck() error {
	start := time.Now()

	// Get running containers
	containers, err := s.dockerClient.GetRunningContainers(s.ctx)
	if err != nil {
		return fmt.Errorf("failed to get running containers: %w", err)
	}

	s.logger.WithField("container_count", len(containers)).Info("Retrieved running containers")

	if len(containers) == 0 {
		s.logger.Info("No running containers found")
		return nil
	}

	// Filter containers based on configuration
	filteredContainers := s.filterContainers(containers)
	s.logger.WithField("filtered_count", len(filteredContainers)).Info("Filtered containers")

	if len(filteredContainers) == 0 {
		s.logger.Info("No containers match the configured filters")
		return nil
	}

	// Build list of images to check
	var imageChecks []registry.ImageCheck
	for _, container := range filteredContainers {
		imageCheck := registry.ImageCheck{
			Registry:   container.Registry,
			Repository: container.Repository,
			Tag:        container.Tag,
		}
		imageChecks = append(imageChecks, imageCheck)
	}

	// Check for updates
	updateResults, err := s.registry.CheckMultipleImages(s.ctx, imageChecks, s.config.App.MaxConcurrency)
	if err != nil {
		s.logger.WithError(err).Error("Failed to check some images for updates")
		// Continue with partial results
	}

	// Filter results that have updates
	var updatesFound []notifications.ImageUpdate
	for _, result := range updateResults {
		if result.HasUpdate {
			// Find corresponding container
			var containerName string
			for _, container := range filteredContainers {
				if container.Registry == result.Registry && container.Repository == result.Repository {
					containerName = container.Name
					break
				}
			}

			update := notifications.ImageUpdate{
				Registry:      result.Registry,
				Repository:    result.Repository,
				CurrentTag:    result.CurrentTag,
				LatestTag:     result.LatestTag,
				ContainerName: containerName,
				UpdateTime:    time.Now(),
			}
			updatesFound = append(updatesFound, update)
		}
	}

	duration := time.Since(start)
	s.logger.WithFields(logrus.Fields{
		"duration":      duration,
		"checked_count": len(imageChecks),
		"updates_found": len(updatesFound),
	}).Info("Completed image check")

	// Send notifications if updates found
	if len(updatesFound) > 0 {
		if err := s.notifications.SendImageUpdates(s.ctx, updatesFound); err != nil {
			s.logger.WithError(err).Error("Failed to send update notifications")
			return err
		}
		s.logger.WithField("update_count", len(updatesFound)).Info("Sent update notifications")
	} else {
		s.logger.Info("No image updates found")
	}

	return nil
}

// filterContainers filters containers based on configuration
func (s *Service) filterContainers(containers []docker.ContainerInfo) []docker.ContainerInfo {
	var filtered []docker.ContainerInfo

	for _, container := range containers {
		// Skip if image should be excluded
		if s.shouldExcludeImage(container.Image) {
			s.logger.WithField("image", container.Image).Debug("Excluding image based on filters")
			continue
		}

		// Skip if include list is specified and image is not included
		if len(s.config.Docker.Filters.Include) > 0 && !s.shouldIncludeImage(container.Image) {
			s.logger.WithField("image", container.Image).Debug("Image not in include list")
			continue
		}

		// Skip latest tags if configured
		if container.Tag == "latest" && !s.config.Docker.Filters.CheckLatest {
			s.logger.WithField("image", container.Image).Debug("Skipping latest tag")
			continue
		}

		// Skip private registries if configured
		imageRef, err := docker.ParseImageReference(container.Image)
		if err != nil {
			s.logger.WithError(err).WithField("image", container.Image).Warn("Failed to parse image reference")
			continue
		}

		if imageRef.IsPrivateRegistry() && !s.config.Docker.Filters.CheckPrivate {
			s.logger.WithField("image", container.Image).Debug("Skipping private registry image")
			continue
		}

		filtered = append(filtered, container)
	}

	return filtered
}

// shouldExcludeImage checks if an image should be excluded
func (s *Service) shouldExcludeImage(image string) bool {
	for _, pattern := range s.config.Docker.Filters.Exclude {
		if matched, _ := matchPattern(pattern, image); matched {
			return true
		}
	}
	return false
}

// shouldIncludeImage checks if an image should be included
func (s *Service) shouldIncludeImage(image string) bool {
	for _, pattern := range s.config.Docker.Filters.Include {
		if matched, _ := matchPattern(pattern, image); matched {
			return true
		}
	}
	return false
}

// matchPattern matches a pattern against a string (simple glob matching)
func matchPattern(pattern, str string) (bool, error) {
	// Simple pattern matching - can be enhanced with filepath.Match or regexp
	if pattern == "*" {
		return true, nil
	}
	return pattern == str, nil
}

// setupScheduledTasks sets up the scheduled image checking tasks
func (s *Service) setupScheduledTasks() error {
	// Convert interval to cron expression
	interval := s.config.GetCheckInterval()
	cronExpr := fmt.Sprintf("@every %s", interval.String())

	// Add image check task
	taskHandler := func(ctx context.Context) error {
		return s.performImageCheck()
	}

	return s.scheduler.AddTask(
		"image-check",
		"Docker Image Update Check",
		cronExpr,
		taskHandler,
	)
}

// setupNotificationChannels sets up notification channels
func setupNotificationChannels(cfg *config.Config, manager *notifications.Manager, logger *logrus.Logger) error {
	// Set up email channel
	if cfg.IsNotificationChannelEnabled("email") {
		emailChannel, err := notifications.NewEmailChannel(notifications.EmailConfig{
			SMTP: notifications.SMTPConfig{
				Host:     cfg.Notifications.Email.SMTP.Host,
				Port:     cfg.Notifications.Email.SMTP.Port,
				Username: cfg.Notifications.Email.SMTP.Username,
				Password: cfg.Notifications.Email.SMTP.Password,
				UseTLS:   cfg.Notifications.Email.SMTP.UseTLS,
			},
			From:    cfg.Notifications.Email.From,
			To:      cfg.Notifications.Email.To,
			Subject: cfg.Notifications.Email.Subject,
			Enabled: true,
		}, logger)
		if err != nil {
			return fmt.Errorf("failed to create email channel: %w", err)
		}

		if err := manager.RegisterChannel(emailChannel); err != nil {
			return fmt.Errorf("failed to register email channel: %w", err)
		}
	}

	// Set up Telegram channel
	if cfg.IsNotificationChannelEnabled("telegram") {
		telegramChannel, err := notifications.NewTelegramChannel(notifications.TelegramConfig{
			BotToken:  cfg.Notifications.Telegram.BotToken,
			ChatIDs:   cfg.Notifications.Telegram.ChatIDs,
			ParseMode: cfg.Notifications.Telegram.ParseMode,
			Enabled:   true,
		}, logger)
		if err != nil {
			return fmt.Errorf("failed to create telegram channel: %w", err)
		}

		if err := manager.RegisterChannel(telegramChannel); err != nil {
			return fmt.Errorf("failed to register telegram channel: %w", err)
		}
	}

	return nil
}

// configureLogger configures the logger based on the configuration
func configureLogger(logger *logrus.Logger, cfg config.LoggingConfig) error {
	// Set log level
	level, err := logrus.ParseLevel(cfg.Level)
	if err != nil {
		return fmt.Errorf("invalid log level: %w", err)
	}
	logger.SetLevel(level)

	// Set log format
	switch cfg.Format {
	case "json":
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: time.RFC3339,
		})
	case "text":
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: time.RFC3339,
		})
	default:
		return fmt.Errorf("unsupported log format: %s", cfg.Format)
	}

	// Set log output
	if cfg.File != "" {
		file, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}
		logger.SetOutput(file)
	}

	return nil
}

// Close closes all service resources
func (s *Service) Close() error {
	if s.cancel != nil {
		s.cancel()
	}

	var errors []error

	if s.dockerClient != nil {
		if err := s.dockerClient.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close Docker client: %w", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors during service cleanup: %v", errors)
	}

	return nil
}
