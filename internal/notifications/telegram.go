package notifications

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sirupsen/logrus"
)

// TelegramChannel handles Telegram notifications
type TelegramChannel struct {
	config TelegramConfig
	logger *logrus.Logger
	bot    *tgbotapi.BotAPI
}

// TelegramConfig contains Telegram configuration
type TelegramConfig struct {
	BotToken  string  `yaml:"bot_token"`
	ChatIDs   []int64 `yaml:"chat_ids"`
	ParseMode string  `yaml:"parse_mode"`
	Enabled   bool    `yaml:"enabled"`
	Template  string  `yaml:"template"`
}

// NewTelegramChannel creates a new Telegram notification channel
func NewTelegramChannel(config TelegramConfig, logger *logrus.Logger) (*TelegramChannel, error) {
	if !config.Enabled {
		return &TelegramChannel{
			config: config,
			logger: logger,
		}, nil
	}

	// Validate configuration
	if config.BotToken == "" {
		return nil, fmt.Errorf("bot token is required")
	}
	if len(config.ChatIDs) == 0 {
		return nil, fmt.Errorf("at least one chat ID is required")
	}

	// Set default parse mode
	if config.ParseMode == "" {
		config.ParseMode = "HTML"
	}

	// Create bot instance
	bot, err := tgbotapi.NewBotAPI(config.BotToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create Telegram bot: %w", err)
	}

	// Test bot connection
	me, err := bot.GetMe()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Telegram API: %w", err)
	}

	logger.WithField("bot_username", me.UserName).Info("Connected to Telegram bot")

	return &TelegramChannel{
		config: config,
		logger: logger,
		bot:    bot,
	}, nil
}

// Send sends a Telegram notification
func (t *TelegramChannel) Send(ctx context.Context, notification *Notification) error {
	if !t.config.Enabled {
		return fmt.Errorf("telegram channel is disabled")
	}

	// Build message text
	messageText := t.buildMessage(notification)

	// Send to all configured chat IDs
	var errors []string
	successCount := 0

	for _, chatID := range t.config.ChatIDs {
		msg := tgbotapi.NewMessage(chatID, messageText)
		msg.ParseMode = t.config.ParseMode

		// Set disable notification for low priority messages
		if notification.Priority == PriorityLow {
			msg.DisableNotification = true
		}

		// Send message with context support
		done := make(chan error, 1)
		go func() {
			_, err := t.bot.Send(msg)
			done <- err
		}()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-done:
			if err != nil {
				t.logger.WithError(err).WithField("chat_id", chatID).
					Error("Failed to send Telegram message")
				errors = append(errors, fmt.Sprintf("chat %d: %v", chatID, err))
			} else {
				t.logger.WithField("chat_id", chatID).
					Debug("Successfully sent Telegram message")
				successCount++
			}
		}
	}

	if successCount == 0 && len(errors) > 0 {
		return fmt.Errorf("failed to send to all chats: %s", strings.Join(errors, "; "))
	}

	if len(errors) > 0 {
		t.logger.WithField("errors", errors).Warn("Some Telegram chats failed")
	}

	t.logger.WithFields(logrus.Fields{
		"chat_ids":      t.config.ChatIDs,
		"success_count": successCount,
		"type":          notification.Type,
	}).Info("Successfully sent Telegram notification")

	return nil
}

// GetType returns the channel type
func (t *TelegramChannel) GetType() string {
	return "telegram"
}

// IsEnabled returns whether the channel is enabled
func (t *TelegramChannel) IsEnabled() bool {
	return t.config.Enabled
}

// buildMessage builds the Telegram message text
func (t *TelegramChannel) buildMessage(notification *Notification) string {
	// Check if we have a custom template
	if t.config.Template != "" {
		return t.renderTemplate(notification)
	}

	// Default template based on notification type
	switch notification.Type {
	case NotificationTypeUpdate:
		return t.buildUpdateMessage(notification)
	case NotificationTypeError:
		return t.buildErrorMessage(notification)
	case NotificationTypeHealth:
		return t.buildHealthMessage(notification)
	default:
		return t.buildGenericMessage(notification)
	}
}

// buildUpdateMessage builds the message for update notifications
func (t *TelegramChannel) buildUpdateMessage(notification *Notification) string {
	var message strings.Builder

	// Header with emoji
	message.WriteString("🐳 <b>Docker Image Updates Available</b>\n\n")

	// Extract updates from data
	if updatesData, ok := notification.Data["updates"]; ok {
		if updates, ok := updatesData.([]ImageUpdate); ok {
			if len(updates) == 1 {
				update := updates[0]
				message.WriteString(fmt.Sprintf("📦 <b>Container:</b> <code>%s</code>\n", update.ContainerName))
				message.WriteString(fmt.Sprintf("🏷️ <b>Image:</b> <code>%s/%s</code>\n", update.Registry, update.Repository))
				message.WriteString(fmt.Sprintf("📊 <b>Current:</b> <code>%s</code>\n", update.CurrentTag))
				message.WriteString(fmt.Sprintf("🆕 <b>Latest:</b> <code>%s</code>\n", update.LatestTag))
				message.WriteString(fmt.Sprintf("🕒 <b>Detected:</b> %s\n\n", update.UpdateTime.Format("2006-01-02 15:04:05")))
			} else {
				message.WriteString(fmt.Sprintf("Found <b>%d</b> image updates:\n\n", len(updates)))

				for i, update := range updates {
					if i >= 10 { // Limit to 10 updates to avoid message length limits
						message.WriteString(fmt.Sprintf("... and %d more updates\n", len(updates)-i))
						break
					}

					message.WriteString(fmt.Sprintf("<b>%d.</b> <code>%s</code>\n", i+1, update.ContainerName))
					message.WriteString(fmt.Sprintf("   📦 <code>%s/%s</code>\n", update.Registry, update.Repository))
					message.WriteString(fmt.Sprintf("   📊 <code>%s</code> → 🆕 <code>%s</code>\n\n", update.CurrentTag, update.LatestTag))
				}
			}
		}
	}

	message.WriteString("💡 <i>Consider updating your containers to get the latest features and security fixes.</i>")

	return message.String()
}

// buildErrorMessage builds the message for error notifications
func (t *TelegramChannel) buildErrorMessage(notification *Notification) string {
	var message strings.Builder

	message.WriteString("⚠️ <b>Docker Notify Error</b>\n\n")

	if context, ok := notification.Data["context"].(string); ok {
		message.WriteString(fmt.Sprintf("📍 <b>Context:</b> <code>%s</code>\n", context))
	}

	if errorMsg, ok := notification.Data["error"].(string); ok {
		// Escape HTML characters in error message
		escapedError := strings.ReplaceAll(errorMsg, "<", "&lt;")
		escapedError = strings.ReplaceAll(escapedError, ">", "&gt;")
		escapedError = strings.ReplaceAll(escapedError, "&", "&amp;")

		message.WriteString(fmt.Sprintf("❌ <b>Error:</b> <code>%s</code>\n\n", escapedError))
	}

	message.WriteString("🔍 <i>Check the Docker Notify service logs for more details.</i>")

	return message.String()
}

// buildHealthMessage builds the message for health notifications
func (t *TelegramChannel) buildHealthMessage(notification *Notification) string {
	var message strings.Builder

	status := "unknown"
	component := "unknown"
	if s, ok := notification.Data["status"].(string); ok {
		status = s
	}
	if c, ok := notification.Data["component"].(string); ok {
		component = c
	}

	// Choose emoji based on status
	emoji := "🏥"
	if status == "healthy" {
		emoji = "✅"
	} else if status == "unhealthy" {
		emoji = "❌"
	}

	message.WriteString(fmt.Sprintf("%s <b>Docker Notify Health Alert</b>\n\n", emoji))
	message.WriteString(fmt.Sprintf("🔧 <b>Component:</b> <code>%s</code>\n", component))
	message.WriteString(fmt.Sprintf("📊 <b>Status:</b> <code>%s</code>\n", strings.ToUpper(status)))

	if details, ok := notification.Data["details"].(string); ok {
		// Escape HTML characters
		escapedDetails := strings.ReplaceAll(details, "<", "&lt;")
		escapedDetails = strings.ReplaceAll(escapedDetails, ">", "&gt;")
		escapedDetails = strings.ReplaceAll(escapedDetails, "&", "&amp;")

		message.WriteString(fmt.Sprintf("📝 <b>Details:</b> <code>%s</code>\n", escapedDetails))
	}

	return message.String()
}

// buildGenericMessage builds a generic message
func (t *TelegramChannel) buildGenericMessage(notification *Notification) string {
	var message strings.Builder

	message.WriteString("📧 <b>Docker Notify</b>\n\n")

	// Escape HTML characters in the message
	escapedMessage := strings.ReplaceAll(notification.Message, "<", "&lt;")
	escapedMessage = strings.ReplaceAll(escapedMessage, ">", "&gt;")
	escapedMessage = strings.ReplaceAll(escapedMessage, "&", "&amp;")

	message.WriteString(escapedMessage)

	return message.String()
}

// renderTemplate renders a custom template (placeholder for future implementation)
func (t *TelegramChannel) renderTemplate(notification *Notification) string {
	// TODO: Implement template rendering with text/template
	return notification.Message
}

// TestConnection tests the Telegram bot connection
func (t *TelegramChannel) TestConnection(ctx context.Context) error {
	if !t.config.Enabled {
		return fmt.Errorf("telegram channel is disabled")
	}

	// Test bot connection
	me, err := t.bot.GetMe()
	if err != nil {
		return fmt.Errorf("failed to connect to Telegram API: %w", err)
	}

	t.logger.WithField("bot_username", me.UserName).Debug("Telegram bot connection test successful")

	// Optionally test sending to first chat ID
	if len(t.config.ChatIDs) > 0 {
		chatID := t.config.ChatIDs[0]

		// Create test message
		testMsg := tgbotapi.NewMessage(chatID, "🧪 <b>Docker Notify Test</b>\n\nThis is a test message to verify the Telegram integration is working correctly.")
		testMsg.ParseMode = t.config.ParseMode
		testMsg.DisableNotification = true

		// Send test message with context support
		done := make(chan error, 1)
		go func() {
			_, err := t.bot.Send(testMsg)
			done <- err
		}()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-done:
			if err != nil {
				return fmt.Errorf("failed to send test message to chat %d: %w", chatID, err)
			}
		}
	}

	return nil
}

// SendTestMessage sends a test message to all configured chats
func (t *TelegramChannel) SendTestMessage(ctx context.Context) error {
	testNotification := &Notification{
		Subject:   "Docker Notify Test",
		Message:   "This is a test notification from Docker Notify service.",
		Timestamp: time.Now(),
		Type:      NotificationTypeInfo,
		Priority:  PriorityNormal,
		Data: map[string]interface{}{
			"test": true,
		},
	}

	return t.Send(ctx, testNotification)
}

// GetBotInfo returns information about the Telegram bot
func (t *TelegramChannel) GetBotInfo() (*tgbotapi.User, error) {
	if !t.config.Enabled || t.bot == nil {
		return nil, fmt.Errorf("telegram channel is not enabled or configured")
	}

	me, err := t.bot.GetMe()
	if err != nil {
		return nil, err
	}
	return &me, nil
}

// GetChatInfo returns information about a specific chat
func (t *TelegramChannel) GetChatInfo(chatID int64) (*tgbotapi.Chat, error) {
	if !t.config.Enabled || t.bot == nil {
		return nil, fmt.Errorf("telegram channel is not enabled or configured")
	}

	chatConfig := tgbotapi.ChatInfoConfig{
		ChatConfig: tgbotapi.ChatConfig{
			ChatID: chatID,
		},
	}

	chat, err := t.bot.GetChat(chatConfig)
	if err != nil {
		return nil, err
	}
	return &chat, nil
}

// FormatChatID formats a chat ID for display
func FormatChatID(chatID int64) string {
	return strconv.FormatInt(chatID, 10)
}

// ParseChatID parses a chat ID from string
func ParseChatID(chatIDStr string) (int64, error) {
	return strconv.ParseInt(chatIDStr, 10, 64)
}
