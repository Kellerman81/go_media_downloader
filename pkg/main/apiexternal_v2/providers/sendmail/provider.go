package sendmail

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
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
func (*Provider) GetProviderType() apiexternal_v2.NotificationProviderType {
	return apiexternal_v2.NotificationSendmail
}

// GetProviderName returns the provider name.
func (*Provider) GetProviderName() string {
	return "sendmail"
}

// stripCRLF removes CR and LF from s, preventing header/command injection
// when s is interpolated into a raw SMTP header line or address list.
func stripCRLF(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	return strings.ReplaceAll(s, "\n", "")
}

// smtpTimeout bounds a whole SMTP exchange when the context has no earlier
// deadline.
const smtpTimeout = 30 * time.Second

// implicitTLSPort is the SMTPS port, where TLS starts before the SMTP greeting.
const implicitTLSPort = 465

// splitRecipients splits a comma-separated address list, trimming entries and
// dropping empty ones.
func splitRecipients(list []string) []string {
	out := make([]string, 0, len(list))
	for i := range list {
		for _, addr := range strings.Split(list[i], ",") {
			if addr = strings.TrimSpace(stripCRLF(addr)); addr != "" {
				out = append(out, addr)
			}
		}
	}

	return out
}

// buildMessage renders the RFC 5322 message: UTF-8 text/plain body in
// quoted-printable, RFC 2047 encoded subject.
func buildMessage(from string, to []string, subject, body string, now time.Time) []byte {
	domain := "localhost"
	if at := strings.LastIndexByte(from, '@'); at >= 0 && at+1 < len(from) {
		domain = strings.TrimRight(from[at+1:], ">")
	}

	var rnd [12]byte
	_, _ = rand.Read(rnd[:])

	var buf bytes.Buffer
	buf.WriteString("From: " + from + "\r\n")
	buf.WriteString("To: " + logger.JoinStringsSep(to, ", ") + "\r\n")
	buf.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", subject) + "\r\n")
	buf.WriteString("Date: " + now.Format(time.RFC1123Z) + "\r\n")
	buf.WriteString("Message-ID: <" + hex.EncodeToString(rnd[:]) + "@" + domain + ">\r\n")
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	buf.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")

	qp := quotedprintable.NewWriter(&buf)
	_, _ = qp.Write([]byte(body))
	_ = qp.Close()

	buf.WriteString("\r\n")

	return buf.Bytes()
}

// dial connects to the SMTP server honouring ctx, using implicit TLS on port
// 465 and STARTTLS when the server offers it, then authenticates if
// credentials are configured. The caller must Close the client.
func (p *Provider) dial(ctx context.Context) (*smtp.Client, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, smtpTimeout)
		defer cancel()
	}

	addr := net.JoinHostPort(p.smtpHost, strconv.Itoa(p.smtpPort))
	tlsConfig := &tls.Config{ServerName: p.smtpHost, MinVersion: tls.VersionTLS12}

	var (
		conn net.Conn
		err  error
	)

	if p.smtpPort == implicitTLSPort {
		d := tls.Dialer{NetDialer: &net.Dialer{}, Config: tlsConfig}
		conn, err = d.DialContext(ctx, "tcp", addr)
	} else {
		var d net.Dialer
		conn, err = d.DialContext(ctx, "tcp", addr)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to SMTP server: %w", err)
	}

	// The deadline bounds the whole exchange, including what the caller does
	// with the returned client; cancellation interrupts the handshake.
	deadline, _ := ctx.Deadline()
	_ = conn.SetDeadline(deadline)

	stop := context.AfterFunc(ctx, func() { _ = conn.SetDeadline(time.Now()) })

	client, err := smtp.NewClient(conn, p.smtpHost)
	if err != nil {
		stop()
		conn.Close()

		return nil, fmt.Errorf("SMTP handshake failed: %w", err)
	}

	fail := func(format string, err error) (*smtp.Client, error) {
		stop()
		client.Close()

		return nil, fmt.Errorf(format, err)
	}

	if p.smtpPort != implicitTLSPort {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(tlsConfig); err != nil {
				return fail("SMTP STARTTLS failed: %w", err)
			}
		}
	}

	if p.username != "" || p.password != "" {
		if ok, _ := client.Extension("AUTH"); ok {
			auth := smtp.PlainAuth("", p.username, p.password, p.smtpHost)
			if err := client.Auth(auth); err != nil {
				return fail("SMTP authentication failed: %w", err)
			}
		} else {
			return fail("SMTP authentication failed: %w", errors.New("server does not offer AUTH"))
		}
	}

	stop()

	return client, nil
}

// SendNotification sends an email notification.
func (p *Provider) SendNotification(
	ctx context.Context,
	request apiexternal_v2.NotificationRequest,
) (*apiexternal_v2.NotificationResponse, error) {
	recipients := p.to
	if request.Options != nil {
		if to, ok := request.Options["to"]; ok {
			recipients = []string{to}
		}
	}

	// Header values must not contain CR/LF - the subject, recipients and
	// p.from end up in the raw header block, and an embedded \r\n would let
	// a caller inject extra headers (e.g. a forged Bcc). splitRecipients
	// builds a new slice, so p.to is never mutated.
	cleanRecipients := splitRecipients(recipients)
	if len(cleanRecipients) == 0 {
		return nil, errors.New("no recipients configured")
	}

	from := stripCRLF(p.from)
	msg := buildMessage(from, cleanRecipients, stripCRLF(request.Title), request.Message, time.Now())

	if err := p.send(ctx, from, cleanRecipients, msg); err != nil {
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

func (p *Provider) send(ctx context.Context, from string, to []string, msg []byte) error {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, smtpTimeout)
		defer cancel()
	}

	client, err := p.dial(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	if err := client.Mail(from); err != nil {
		return err
	}

	for i := range to {
		if err := client.Rcpt(to[i]); err != nil {
			return err
		}
	}

	w, err := client.Data()
	if err != nil {
		return err
	}

	if _, err := w.Write(msg); err != nil {
		return err
	}

	if err := w.Close(); err != nil {
		return err
	}

	return client.Quit()
}

// TestConnection validates the SMTP server connection, including STARTTLS
// and authentication when credentials are configured.
func (p *Provider) TestConnection(ctx context.Context) error {
	client, err := p.dial(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	return client.Quit()
}
