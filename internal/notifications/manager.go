package notifications

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Manager handles all notification operations
type Manager struct {
	channels map[string]Channel
	logger   *logrus.Logger
	mu       sync.RWMutex
}

// Channel represents a notification channel interface
type Channel interface {
	Send(ctx context.Context, notification *Notification) error
	GetType() string
	IsEnabled() bool
}

// Notification represents a notification message
type Notification struct {
	Subject   string                 `json:"subject"`
	Message   string                 `json:"message"`
	Timestamp time.Time              `json:"timestamp"`
	Type      NotificationType       `json:"type"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Priority  Priority               `json:"priority"`
}

// NotificationType represents the type of notification
type NotificationType string

const (
	NotificationTypeUpdate NotificationType = "update"
	NotificationTypeError  NotificationType = "error"
	NotificationTypeInfo   NotificationType = "info"
	NotificationTypeHealth NotificationType = "health"
)

// Priority represents notification priority
type Priority string

const (
	PriorityLow      Priority = "low"
	PriorityNormal   Priority = "normal"
	PriorityHigh     Priority = "high"
	PriorityCritical Priority = "critical"
)

// ImageUpdate represents an image update notification data
type ImageUpdate struct {
	Registry      string    `json:"registry"`
	Repository    string    `json:"repository"`
	CurrentTag    string    `json:"current_tag"`
	LatestTag     string    `json:"latest_tag"`
	ContainerName string    `json:"container_name"`
	UpdateTime    time.Time `json:"update_time"`
}

// NewManager creates a new notification manager
func NewManager(logger *logrus.Logger) *Manager {
	return &Manager{
		channels: make(map[string]Channel),
		logger:   logger,
	}
}

// RegisterChannel registers a notification channel
func (m *Manager) RegisterChannel(channel Channel) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	channelType := channel.GetType()
	if _, exists := m.channels[channelType]; exists {
		return fmt.Errorf("channel type %s already registered", channelType)
	}

	m.channels[channelType] = channel
	m.logger.WithField("channel_type", channelType).Info("Registered notification channel")
	return nil
}

// UnregisterChannel unregisters a notification channel
func (m *Manager) UnregisterChannel(channelType string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.channels, channelType)
	m.logger.WithField("channel_type", channelType).Info("Unregistered notification channel")
}

// Send sends a notification to all enabled channels
func (m *Manager) Send(ctx context.Context, notification *Notification) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.channels) == 0 {
		m.logger.Warn("No notification channels registered")
		return fmt.Errorf("no notification channels available")
	}

	var errors []string
	successCount := 0

	for channelType, channel := range m.channels {
		if !channel.IsEnabled() {
			m.logger.WithField("channel_type", channelType).Debug("Channel is disabled, skipping")
			continue
		}

		if err := channel.Send(ctx, notification); err != nil {
			m.logger.WithError(err).WithField("channel_type", channelType).
				Error("Failed to send notification")
			errors = append(errors, fmt.Sprintf("%s: %v", channelType, err))
		} else {
			m.logger.WithField("channel_type", channelType).
				Debug("Successfully sent notification")
			successCount++
		}
	}

	if successCount == 0 && len(errors) > 0 {
		return fmt.Errorf("all notification channels failed: %s", strings.Join(errors, "; "))
	}

	if len(errors) > 0 {
		m.logger.WithField("errors", errors).Warn("Some notification channels failed")
	}

	return nil
}

// SendImageUpdates sends notifications about image updates
func (m *Manager) SendImageUpdates(ctx context.Context, updates []ImageUpdate) error {
	if len(updates) == 0 {
		return nil
	}

	// Create notification
	notification := &Notification{
		Subject:   m.buildUpdateSubject(updates),
		Message:   m.buildUpdateMessage(updates),
		Timestamp: time.Now(),
		Type:      NotificationTypeUpdate,
		Priority:  PriorityNormal,
		Data: map[string]interface{}{
			"updates": updates,
			"count":   len(updates),
		},
	}

	return m.Send(ctx, notification)
}

// SendError sends an error notification
func (m *Manager) SendError(ctx context.Context, err error, context string) error {
	notification := &Notification{
		Subject:   fmt.Sprintf("Docker Notify Error: %s", context),
		Message:   fmt.Sprintf("An error occurred in Docker Notify:\n\nContext: %s\nError: %s", context, err.Error()),
		Timestamp: time.Now(),
		Type:      NotificationTypeError,
		Priority:  PriorityHigh,
		Data: map[string]interface{}{
			"error":   err.Error(),
			"context": context,
		},
	}

	return m.Send(ctx, notification)
}

// SendHealthAlert sends a health alert notification
func (m *Manager) SendHealthAlert(ctx context.Context, component string, status string, details string) error {
	priority := PriorityNormal
	if status == "unhealthy" {
		priority = PriorityHigh
	}

	notification := &Notification{
		Subject:   fmt.Sprintf("Docker Notify Health Alert: %s is %s", component, status),
		Message:   fmt.Sprintf("Health check for %s returned status: %s\n\nDetails: %s", component, status, details),
		Timestamp: time.Now(),
		Type:      NotificationTypeHealth,
		Priority:  priority,
		Data: map[string]interface{}{
			"component": component,
			"status":    status,
			"details":   details,
		},
	}

	return m.Send(ctx, notification)
}

// buildUpdateSubject builds the subject line for update notifications
func (m *Manager) buildUpdateSubject(updates []ImageUpdate) string {
	if len(updates) == 1 {
		update := updates[0]
		return fmt.Sprintf("Docker Image Update Available: %s:%s → %s",
			update.Repository, update.CurrentTag, update.LatestTag)
	}
	return fmt.Sprintf("Docker Image Updates Available (%d images)", len(updates))
}

// buildUpdateMessage builds the message body for update notifications
func (m *Manager) buildUpdateMessage(updates []ImageUpdate) string {
	var message strings.Builder

	if len(updates) == 1 {
		update := updates[0]
		message.WriteString("A newer version of the Docker image is available:\n\n")
		message.WriteString(fmt.Sprintf("🐳 **Image:** %s/%s\n", update.Registry, update.Repository))
		message.WriteString(fmt.Sprintf("📦 **Container:** %s\n", update.ContainerName))
		message.WriteString(fmt.Sprintf("📊 **Current Version:** %s\n", update.CurrentTag))
		message.WriteString(fmt.Sprintf("🆕 **Latest Version:** %s\n", update.LatestTag))
		message.WriteString(fmt.Sprintf("🕒 **Detected:** %s\n\n", update.UpdateTime.Format("2006-01-02 15:04:05")))
		message.WriteString("Consider updating your container to get the latest features and security fixes.")
	} else {
		message.WriteString("Multiple Docker images have updates available:\n\n")

		for i, update := range updates {
			message.WriteString(fmt.Sprintf("**%d. %s/%s**\n", i+1, update.Registry, update.Repository))
			message.WriteString(fmt.Sprintf("   📦 Container: %s\n", update.ContainerName))
			message.WriteString(fmt.Sprintf("   📊 %s → 🆕 %s\n", update.CurrentTag, update.LatestTag))
			message.WriteString(fmt.Sprintf("   🕒 %s\n\n", update.UpdateTime.Format("2006-01-02 15:04:05")))
		}

		message.WriteString("Consider updating these containers to get the latest features and security fixes.")
	}

	return message.String()
}

// GetRegisteredChannels returns a list of registered channel types
func (m *Manager) GetRegisteredChannels() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	channels := make([]string, 0, len(m.channels))
	for channelType := range m.channels {
		channels = append(channels, channelType)
	}
	return channels
}

// GetEnabledChannels returns a list of enabled channel types
func (m *Manager) GetEnabledChannels() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var enabled []string
	for channelType, channel := range m.channels {
		if channel.IsEnabled() {
			enabled = append(enabled, channelType)
		}
	}
	return enabled
}

// Health checks the health of all notification channels
func (m *Manager) Health(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.channels) == 0 {
		return fmt.Errorf("no notification channels registered")
	}

	enabledCount := 0
	for _, channel := range m.channels {
		if channel.IsEnabled() {
			enabledCount++
		}
	}

	if enabledCount == 0 {
		return fmt.Errorf("no notification channels are enabled")
	}

	return nil
}
