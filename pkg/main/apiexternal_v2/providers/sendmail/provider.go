package sendmail

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

//
// Sendmail Provider - Email notification service
// Fully typed implementation with BaseClient infrastructure
//

// Provider implements the NotificationProvider interface for email.
type Provider struct {
	*base.BaseClient
	smtpHost string
	smtpPort int
	from     string
	to       []string
	username string
	password string
}

// NewProvider creates a new Sendmail notification provider.
func NewProvider(
	smtpHost string,
	smtpPort int,
	from string,
	to []string,
	username, password string,
) *Provider {
	config := base.ClientConfig{
		Name:                    "sendmail",
		BaseURL:                 fmt.Sprintf("%s:%d", smtpHost, smtpPort),
		Timeout:                 30 * time.Second,
		AuthType:                base.AuthBasic,
		Username:                username,
		Password:                password,
		CircuitBreakerThreshold: 5,
		CircuitBreakerTimeout:   60 * time.Second,
		EnableStats:             true,
		StatsDBTable:            "api_client_stats",
		MaxRetries:              3,
		RetryBackoff:            2 * time.Second,
	}

	return &Provider{
		BaseClient: base.NewBaseClient(config),
		smtpHost:   smtpHost,
		smtpPort:   smtpPort,
		from:       from,
		to:         to,
		username:   username,
		password:   password,
	}
}

// GetProviderType returns the provider type.
func (p *Provider) GetProviderType() apiexternal_v2.NotificationProviderType {
	return apiexternal_v2.NotificationSendmail
}

// GetProviderName returns the provider name.
func (p *Provider) GetProviderName() string {
	return "sendmail"
}

// SendNotification sends an email notification.
func (p *Provider) SendNotification(
	ctx context.Context,
	request apiexternal_v2.NotificationRequest,
) (*apiexternal_v2.NotificationResponse, error) {
	recipients := p.to
	if request.Options != nil {
		if to, ok := request.Options["to"]; ok {
			recipients = strings.Split(to, ",")
		}
	}

	if len(recipients) == 0 {
		return nil, fmt.Errorf("no recipients configured")
	}

	// Build email message
	subject := request.Title
	body := request.Message

	msg := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s\r\n",
		p.from,
		strings.Join(recipients, ", "),
		subject,
		body,
	))

	// Setup authentication
	auth := smtp.PlainAuth("", p.username, p.password, p.smtpHost)

	// Send email
	addr := fmt.Sprintf("%s:%d", p.smtpHost, p.smtpPort)

	err := smtp.SendMail(addr, auth, p.from, recipients, msg)
	if err != nil {
		return &apiexternal_v2.NotificationResponse{
			Success:   false,
			Timestamp: time.Now(),
			Provider:  "sendmail",
			Error:     err.Error(),
		}, fmt.Errorf("failed to send email: %w", err)
	}

	return &apiexternal_v2.NotificationResponse{
		Success:   true,
		Timestamp: time.Now(),
		Provider:  "sendmail",
	}, nil
}

// TestConnection validates the SMTP server connection.
func (p *Provider) TestConnection(ctx context.Context) error {
	// Try to connect to SMTP server
	client, err := smtp.Dial(fmt.Sprintf("%s:%d", p.smtpHost, p.smtpPort))
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer client.Close()

	// Try to authenticate
	if p.username != "" && p.password != "" {
		auth := smtp.PlainAuth("", p.username, p.password, p.smtpHost)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP authentication failed: %w", err)
		}
	}

	return nil
}
