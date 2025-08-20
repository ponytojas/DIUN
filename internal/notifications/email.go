package notifications

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"gopkg.in/gomail.v2"
)

// EmailChannel handles email notifications
type EmailChannel struct {
	config EmailConfig
	logger *logrus.Logger
	dialer *gomail.Dialer
}

// EmailConfig contains email configuration
type EmailConfig struct {
	SMTP     SMTPConfig `yaml:"smtp"`
	From     string     `yaml:"from"`
	To       []string   `yaml:"to"`
	Subject  string     `yaml:"subject"`
	Enabled  bool       `yaml:"enabled"`
	Template string     `yaml:"template"`
}

// SMTPConfig contains SMTP server configuration
type SMTPConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	UseTLS   bool   `yaml:"use_tls"`
}

// NewEmailChannel creates a new email notification channel
func NewEmailChannel(config EmailConfig, logger *logrus.Logger) (*EmailChannel, error) {
	if !config.Enabled {
		return &EmailChannel{
			config: config,
			logger: logger,
		}, nil
	}

	// Validate configuration
	if config.SMTP.Host == "" {
		return nil, fmt.Errorf("SMTP host is required")
	}
	if config.SMTP.Port == 0 {
		return nil, fmt.Errorf("SMTP port is required")
	}
	if config.From == "" {
		return nil, fmt.Errorf("from address is required")
	}
	if len(config.To) == 0 {
		return nil, fmt.Errorf("at least one recipient is required")
	}

	// Create SMTP dialer
	dialer := gomail.NewDialer(
		config.SMTP.Host,
		config.SMTP.Port,
		config.SMTP.Username,
		config.SMTP.Password,
	)

	// Configure TLS
	if config.SMTP.UseTLS {
		dialer.TLSConfig = &tls.Config{
			ServerName: config.SMTP.Host,
		}
	} else {
		dialer.TLSConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	return &EmailChannel{
		config: config,
		logger: logger,
		dialer: dialer,
	}, nil
}

// Send sends an email notification
func (e *EmailChannel) Send(ctx context.Context, notification *Notification) error {
	if !e.config.Enabled {
		return fmt.Errorf("email channel is disabled")
	}

	// Create message
	message := gomail.NewMessage()

	// Set headers
	message.SetHeader("From", e.config.From)
	message.SetHeader("To", e.config.To...)
	message.SetHeader("Subject", e.buildSubject(notification))

	// Set body based on notification type
	body := e.buildBody(notification)
	if e.isHTMLContent(body) {
		message.SetBody("text/html", body)
	} else {
		message.SetBody("text/plain", body)
	}

	// Add priority header if high priority
	if notification.Priority == PriorityHigh || notification.Priority == PriorityCritical {
		message.SetHeader("X-Priority", "1")
		message.SetHeader("Importance", "high")
	}

	// Add custom headers
	message.SetHeader("X-Docker-Notify", "true")
	message.SetHeader("X-Notification-Type", string(notification.Type))
	message.SetHeader("X-Notification-Priority", string(notification.Priority))

	// Send email with context cancellation support
	done := make(chan error, 1)
	go func() {
		done <- e.dialer.DialAndSend(message)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		if err != nil {
			e.logger.WithError(err).Error("Failed to send email notification")
			return fmt.Errorf("failed to send email: %w", err)
		}
	}

	e.logger.WithFields(logrus.Fields{
		"to":      e.config.To,
		"subject": message.GetHeader("Subject"),
		"type":    notification.Type,
	}).Info("Successfully sent email notification")

	return nil
}

// GetType returns the channel type
func (e *EmailChannel) GetType() string {
	return "email"
}

// IsEnabled returns whether the channel is enabled
func (e *EmailChannel) IsEnabled() bool {
	return e.config.Enabled
}

// buildSubject builds the email subject
func (e *EmailChannel) buildSubject(notification *Notification) string {
	if e.config.Subject != "" && notification.Subject != "" {
		return fmt.Sprintf("%s: %s", e.config.Subject, notification.Subject)
	}
	if e.config.Subject != "" {
		return e.config.Subject
	}
	if notification.Subject != "" {
		return notification.Subject
	}
	return "Docker Notify Alert"
}

// buildBody builds the email body
func (e *EmailChannel) buildBody(notification *Notification) string {
	var body strings.Builder

	// Check if we have a custom template
	if e.config.Template != "" {
		return e.renderTemplate(notification)
	}

	// Default template based on notification type
	switch notification.Type {
	case NotificationTypeUpdate:
		body.WriteString(e.buildUpdateEmailBody(notification))
	case NotificationTypeError:
		body.WriteString(e.buildErrorEmailBody(notification))
	case NotificationTypeHealth:
		body.WriteString(e.buildHealthEmailBody(notification))
	default:
		body.WriteString(e.buildGenericEmailBody(notification))
	}

	return body.String()
}

// buildUpdateEmailBody builds the body for update notifications
func (e *EmailChannel) buildUpdateEmailBody(notification *Notification) string {
	var body strings.Builder

	body.WriteString("<!DOCTYPE html>\n")
	body.WriteString("<html>\n<head>\n")
	body.WriteString("<style>\n")
	body.WriteString("body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }\n")
	body.WriteString(".container { max-width: 600px; margin: 0 auto; padding: 20px; }\n")
	body.WriteString(".header { background-color: #2196F3; color: white; padding: 20px; text-align: center; }\n")
	body.WriteString(".content { padding: 20px; background-color: #f9f9f9; }\n")
	body.WriteString(".update-item { background-color: white; margin: 10px 0; padding: 15px; border-left: 4px solid #2196F3; }\n")
	body.WriteString(".footer { text-align: center; padding: 20px; color: #666; font-size: 12px; }\n")
	body.WriteString("</style>\n")
	body.WriteString("</head>\n<body>\n")

	body.WriteString("<div class=\"container\">\n")
	body.WriteString("<div class=\"header\">\n")
	body.WriteString("<h1>🐳 Docker Image Updates Available</h1>\n")
	body.WriteString("</div>\n")

	body.WriteString("<div class=\"content\">\n")
	body.WriteString("<p>New versions of your Docker images are available:</p>\n")

	// Extract updates from data
	if updatesData, ok := notification.Data["updates"]; ok {
		if updates, ok := updatesData.([]ImageUpdate); ok {
			for _, update := range updates {
				body.WriteString("<div class=\"update-item\">\n")
				body.WriteString(fmt.Sprintf("<h3>%s/%s</h3>\n", update.Registry, update.Repository))
				body.WriteString(fmt.Sprintf("<p><strong>Container:</strong> %s</p>\n", update.ContainerName))
				body.WriteString(fmt.Sprintf("<p><strong>Current:</strong> %s → <strong>Latest:</strong> %s</p>\n",
					update.CurrentTag, update.LatestTag))
				body.WriteString(fmt.Sprintf("<p><strong>Detected:</strong> %s</p>\n",
					update.UpdateTime.Format("2006-01-02 15:04:05")))
				body.WriteString("</div>\n")
			}
		}
	}

	body.WriteString("<p>Consider updating your containers to get the latest features and security fixes.</p>\n")
	body.WriteString("</div>\n")

	body.WriteString("<div class=\"footer\">\n")
	body.WriteString("<p>This notification was sent by Docker Notify</p>\n")
	body.WriteString(fmt.Sprintf("<p>Generated at: %s</p>\n", notification.Timestamp.Format("2006-01-02 15:04:05 UTC")))
	body.WriteString("</div>\n")

	body.WriteString("</div>\n")
	body.WriteString("</body>\n</html>")

	return body.String()
}

// buildErrorEmailBody builds the body for error notifications
func (e *EmailChannel) buildErrorEmailBody(notification *Notification) string {
	var body strings.Builder

	body.WriteString("<!DOCTYPE html>\n")
	body.WriteString("<html>\n<head>\n")
	body.WriteString("<style>\n")
	body.WriteString("body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }\n")
	body.WriteString(".container { max-width: 600px; margin: 0 auto; padding: 20px; }\n")
	body.WriteString(".header { background-color: #f44336; color: white; padding: 20px; text-align: center; }\n")
	body.WriteString(".content { padding: 20px; background-color: #f9f9f9; }\n")
	body.WriteString(".error-box { background-color: #ffebee; border: 1px solid #f44336; padding: 15px; margin: 10px 0; }\n")
	body.WriteString(".footer { text-align: center; padding: 20px; color: #666; font-size: 12px; }\n")
	body.WriteString("</style>\n")
	body.WriteString("</head>\n<body>\n")

	body.WriteString("<div class=\"container\">\n")
	body.WriteString("<div class=\"header\">\n")
	body.WriteString("<h1>⚠️ Docker Notify Error</h1>\n")
	body.WriteString("</div>\n")

	body.WriteString("<div class=\"content\">\n")
	body.WriteString("<p>An error occurred in the Docker Notify service:</p>\n")

	body.WriteString("<div class=\"error-box\">\n")
	if context, ok := notification.Data["context"].(string); ok {
		body.WriteString(fmt.Sprintf("<p><strong>Context:</strong> %s</p>\n", context))
	}
	if errorMsg, ok := notification.Data["error"].(string); ok {
		body.WriteString(fmt.Sprintf("<p><strong>Error:</strong> %s</p>\n", errorMsg))
	}
	body.WriteString("</div>\n")

	body.WriteString("<p>Please check the Docker Notify service logs for more details.</p>\n")
	body.WriteString("</div>\n")

	body.WriteString("<div class=\"footer\">\n")
	body.WriteString("<p>This notification was sent by Docker Notify</p>\n")
	body.WriteString(fmt.Sprintf("<p>Generated at: %s</p>\n", notification.Timestamp.Format("2006-01-02 15:04:05 UTC")))
	body.WriteString("</div>\n")

	body.WriteString("</div>\n")
	body.WriteString("</body>\n</html>")

	return body.String()
}

// buildHealthEmailBody builds the body for health notifications
func (e *EmailChannel) buildHealthEmailBody(notification *Notification) string {
	var body strings.Builder

	status := "unknown"
	component := "unknown"
	if s, ok := notification.Data["status"].(string); ok {
		status = s
	}
	if c, ok := notification.Data["component"].(string); ok {
		component = c
	}

	color := "#4CAF50" // green for healthy
	if status == "unhealthy" {
		color = "#f44336" // red for unhealthy
	}

	body.WriteString("<!DOCTYPE html>\n")
	body.WriteString("<html>\n<head>\n")
	body.WriteString("<style>\n")
	body.WriteString("body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }\n")
	body.WriteString(".container { max-width: 600px; margin: 0 auto; padding: 20px; }\n")
	body.WriteString(fmt.Sprintf(".header { background-color: %s; color: white; padding: 20px; text-align: center; }\n", color))
	body.WriteString(".content { padding: 20px; background-color: #f9f9f9; }\n")
	body.WriteString(".status-box { background-color: white; border-left: 4px solid " + color + "; padding: 15px; margin: 10px 0; }\n")
	body.WriteString(".footer { text-align: center; padding: 20px; color: #666; font-size: 12px; }\n")
	body.WriteString("</style>\n")
	body.WriteString("</head>\n<body>\n")

	body.WriteString("<div class=\"container\">\n")
	body.WriteString("<div class=\"header\">\n")
	body.WriteString("<h1>🏥 Docker Notify Health Alert</h1>\n")
	body.WriteString("</div>\n")

	body.WriteString("<div class=\"content\">\n")
	body.WriteString("<div class=\"status-box\">\n")
	body.WriteString(fmt.Sprintf("<h3>Component: %s</h3>\n", component))
	body.WriteString(fmt.Sprintf("<p><strong>Status:</strong> %s</p>\n", strings.ToUpper(status)))
	if details, ok := notification.Data["details"].(string); ok {
		body.WriteString(fmt.Sprintf("<p><strong>Details:</strong> %s</p>\n", details))
	}
	body.WriteString("</div>\n")
	body.WriteString("</div>\n")

	body.WriteString("<div class=\"footer\">\n")
	body.WriteString("<p>This notification was sent by Docker Notify</p>\n")
	body.WriteString(fmt.Sprintf("<p>Generated at: %s</p>\n", notification.Timestamp.Format("2006-01-02 15:04:05 UTC")))
	body.WriteString("</div>\n")

	body.WriteString("</div>\n")
	body.WriteString("</body>\n</html>")

	return body.String()
}

// buildGenericEmailBody builds a generic email body
func (e *EmailChannel) buildGenericEmailBody(notification *Notification) string {
	var body strings.Builder

	body.WriteString("<!DOCTYPE html>\n")
	body.WriteString("<html>\n<head>\n")
	body.WriteString("<style>\n")
	body.WriteString("body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }\n")
	body.WriteString(".container { max-width: 600px; margin: 0 auto; padding: 20px; }\n")
	body.WriteString(".header { background-color: #607D8B; color: white; padding: 20px; text-align: center; }\n")
	body.WriteString(".content { padding: 20px; background-color: #f9f9f9; }\n")
	body.WriteString(".footer { text-align: center; padding: 20px; color: #666; font-size: 12px; }\n")
	body.WriteString("</style>\n")
	body.WriteString("</head>\n<body>\n")

	body.WriteString("<div class=\"container\">\n")
	body.WriteString("<div class=\"header\">\n")
	body.WriteString("<h1>📧 Docker Notify</h1>\n")
	body.WriteString("</div>\n")

	body.WriteString("<div class=\"content\">\n")
	body.WriteString(fmt.Sprintf("<p>%s</p>\n", notification.Message))
	body.WriteString("</div>\n")

	body.WriteString("<div class=\"footer\">\n")
	body.WriteString("<p>This notification was sent by Docker Notify</p>\n")
	body.WriteString(fmt.Sprintf("<p>Generated at: %s</p>\n", notification.Timestamp.Format("2006-01-02 15:04:05 UTC")))
	body.WriteString("</div>\n")

	body.WriteString("</div>\n")
	body.WriteString("</body>\n</html>")

	return body.String()
}

// renderTemplate renders a custom template (placeholder for future implementation)
func (e *EmailChannel) renderTemplate(notification *Notification) string {
	// TODO: Implement template rendering with text/template or html/template
	return notification.Message
}

// isHTMLContent checks if the content contains HTML tags
func (e *EmailChannel) isHTMLContent(content string) bool {
	return strings.Contains(content, "<html>") || strings.Contains(content, "<!DOCTYPE")
}

// TestConnection tests the SMTP connection
func (e *EmailChannel) TestConnection(ctx context.Context) error {
	if !e.config.Enabled {
		return fmt.Errorf("email channel is disabled")
	}

	// Create a test message
	message := gomail.NewMessage()
	message.SetHeader("From", e.config.From)
	message.SetHeader("To", e.config.To[0])
	message.SetHeader("Subject", "Docker Notify Test")
	message.SetBody("text/plain", "This is a test message from Docker Notify.")

	// Test connection without sending
	closer, err := e.dialer.Dial()
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer closer.Close()

	return nil
}
