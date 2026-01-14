package services

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/tesseract-hub/go-shared/security"
)

// PostalProvider implements email sending via Postal SMTP
// Postal is a self-hosted mail delivery platform (https://postal.atech.media/)
type PostalProvider struct {
	host     string
	port     string
	username string
	password string
	from     string
	fromName string
}

// NewPostalProvider creates a new Postal SMTP provider
func NewPostalProvider(config *ProviderConfig) *PostalProvider {
	return &PostalProvider{
		host:     config.PostalHost,
		port:     fmt.Sprintf("%d", config.PostalPort),
		username: config.PostalUsername,
		password: config.PostalPassword,
		from:     config.PostalFrom,
		fromName: config.PostalFromName,
	}
}

// Send sends an email via Postal SMTP
func (p *PostalProvider) Send(ctx context.Context, message *Message) (*SendResult, error) {
	startTime := time.Now()
	log.Printf("[POSTAL] Sending email to %s, subject: %s", security.MaskEmail(message.To), message.Subject)

	// Build from address
	from := p.from
	if p.fromName != "" {
		from = fmt.Sprintf("%s <%s>", p.fromName, p.from)
	}

	// Override from if provided in message
	if message.From != "" {
		from = message.From
		if message.FromName != "" {
			from = fmt.Sprintf("%s <%s>", message.FromName, message.From)
		}
	}

	// Prepare headers
	headers := make(map[string]string)
	headers["From"] = from
	headers["To"] = message.To
	headers["Subject"] = message.Subject
	headers["MIME-Version"] = "1.0"
	headers["Date"] = time.Now().Format(time.RFC1123Z)
	headers["X-Mailer"] = "Tesseract-Hub/1.0"

	// Add CC and BCC if provided
	if len(message.CC) > 0 {
		headers["Cc"] = strings.Join(message.CC, ", ")
	}

	// Add Reply-To if provided
	if message.ReplyTo != "" {
		headers["Reply-To"] = message.ReplyTo
	}

	// Add custom headers
	for key, value := range message.Headers {
		headers[key] = value
	}

	// Build message body with proper MIME structure
	var emailBuilder strings.Builder

	// Add headers
	for k, v := range headers {
		emailBuilder.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}

	// Handle HTML and plain text with multipart if both exist
	if message.BodyHTML != "" && message.Body != "" {
		boundary := fmt.Sprintf("----=_Part_%d", time.Now().UnixNano())
		headers["Content-Type"] = fmt.Sprintf("multipart/alternative; boundary=\"%s\"", boundary)

		emailBuilder.Reset()
		for k, v := range headers {
			emailBuilder.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
		}
		emailBuilder.WriteString("\r\n")
		emailBuilder.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		emailBuilder.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
		emailBuilder.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
		emailBuilder.WriteString(message.Body)
		emailBuilder.WriteString(fmt.Sprintf("\r\n\r\n--%s\r\n", boundary))
		emailBuilder.WriteString("Content-Type: text/html; charset=utf-8\r\n")
		emailBuilder.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
		emailBuilder.WriteString(message.BodyHTML)
		emailBuilder.WriteString(fmt.Sprintf("\r\n\r\n--%s--\r\n", boundary))
	} else if message.BodyHTML != "" {
		headers["Content-Type"] = "text/html; charset=utf-8"
		emailBuilder.Reset()
		for k, v := range headers {
			emailBuilder.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
		}
		emailBuilder.WriteString("\r\n")
		emailBuilder.WriteString(message.BodyHTML)
	} else {
		headers["Content-Type"] = "text/plain; charset=utf-8"
		emailBuilder.Reset()
		for k, v := range headers {
			emailBuilder.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
		}
		emailBuilder.WriteString("\r\n")
		emailBuilder.WriteString(message.Body)
	}

	// Collect all recipients
	recipients := []string{message.To}
	recipients = append(recipients, message.CC...)
	recipients = append(recipients, message.BCC...)

	// Send email
	addr := net.JoinHostPort(p.host, p.port)

	// Try STARTTLS first (port 587), then direct TLS (port 465), then plain (port 25)
	var sendErr error

	// Determine connection method based on port
	switch p.port {
	case "465":
		// Direct TLS connection
		sendErr = p.sendWithTLS(addr, recipients, emailBuilder.String())
	case "587":
		// STARTTLS
		sendErr = p.sendWithSTARTTLS(addr, recipients, emailBuilder.String())
	default:
		// Try plain SMTP (port 25)
		sendErr = p.sendPlain(addr, recipients, emailBuilder.String())
	}

	if sendErr != nil {
		log.Printf("[POSTAL] Failed to send email: %v (took %v)", sendErr, time.Since(startTime))
		return &SendResult{
			ProviderName: "Postal",
			Success:      false,
			Error:        sendErr,
		}, sendErr
	}

	log.Printf("[POSTAL] Email sent successfully to %s (took %v)", security.MaskEmail(message.To), time.Since(startTime))
	return &SendResult{
		ProviderName: "Postal",
		Success:      true,
		ProviderData: map[string]interface{}{
			"to":       message.To,
			"subject":  message.Subject,
			"duration": time.Since(startTime).String(),
		},
	}, nil
}

// sendWithTLS sends email using direct TLS connection (port 465)
func (p *PostalProvider) sendWithTLS(addr string, recipients []string, body string) error {
	tlsConfig := &tls.Config{
		ServerName:         p.host,
		InsecureSkipVerify: false,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("TLS dial failed: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, p.host)
	if err != nil {
		return fmt.Errorf("SMTP client creation failed: %w", err)
	}
	defer client.Quit()

	return p.sendViaClient(client, recipients, body)
}

// sendWithSTARTTLS sends email using STARTTLS (port 587)
func (p *PostalProvider) sendWithSTARTTLS(addr string, recipients []string, body string) error {
	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("SMTP dial failed: %w", err)
	}
	defer client.Quit()

	// Send EHLO
	if err = client.Hello("localhost"); err != nil {
		return fmt.Errorf("EHLO failed: %w", err)
	}

	// Start TLS
	tlsConfig := &tls.Config{
		ServerName:         p.host,
		InsecureSkipVerify: false,
	}
	if err = client.StartTLS(tlsConfig); err != nil {
		return fmt.Errorf("STARTTLS failed: %w", err)
	}

	return p.sendViaClient(client, recipients, body)
}

// sendPlain sends email using SMTP port 25 with STARTTLS (required for auth)
// Go's smtp.PlainAuth refuses to send credentials over unencrypted connections,
// so we establish STARTTLS first even on port 25
func (p *PostalProvider) sendPlain(addr string, recipients []string, body string) error {
	// Connect to SMTP server
	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("SMTP dial failed: %w", err)
	}
	defer client.Quit()

	// Send EHLO
	if err = client.Hello("notification-service.devtest.svc.cluster.local"); err != nil {
		return fmt.Errorf("EHLO failed: %w", err)
	}

	// Try STARTTLS if available (required for PlainAuth)
	if ok, _ := client.Extension("STARTTLS"); ok {
		tlsConfig := &tls.Config{
			ServerName:         p.host,
			InsecureSkipVerify: true, // Allow self-signed certs for internal cluster
		}
		if err = client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("STARTTLS failed: %w", err)
		}
	}

	return p.sendViaClient(client, recipients, body)
}

// sendViaClient sends email using an established SMTP client
func (p *PostalProvider) sendViaClient(client *smtp.Client, recipients []string, body string) error {
	// Authenticate using LOGIN auth (more compatible than PLAIN)
	// Postal and many SMTP servers prefer LOGIN auth
	auth := &loginAuth{username: p.username, password: p.password}
	if err := client.Auth(auth); err != nil {
		// Fall back to PlainAuth if LOGIN fails
		plainAuth := smtp.PlainAuth("", p.username, p.password, p.host)
		if err2 := client.Auth(plainAuth); err2 != nil {
			return fmt.Errorf("auth failed (LOGIN: %v, PLAIN: %v)", err, err2)
		}
	}

	// Set sender
	if err := client.Mail(p.from); err != nil {
		return fmt.Errorf("MAIL FROM failed: %w", err)
	}

	// Set recipients
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("RCPT TO failed for %s: %w", recipient, err)
		}
	}

	// Send body
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA command failed: %w", err)
	}

	_, err = w.Write([]byte(body))
	if err != nil {
		return fmt.Errorf("write body failed: %w", err)
	}

	if err = w.Close(); err != nil {
		return fmt.Errorf("close data writer failed: %w", err)
	}

	return nil
}

// GetName returns the provider name
func (p *PostalProvider) GetName() string {
	return "Postal"
}

// SupportsChannel returns the supported channel
func (p *PostalProvider) SupportsChannel() string {
	return "EMAIL"
}

// loginAuth implements SMTP LOGIN authentication
// This is more compatible with Postal and other SMTP servers than PLAIN auth
type loginAuth struct {
	username, password string
}

func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", nil, nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}

	switch string(fromServer) {
	case "Username:", "Username":
		return []byte(a.username), nil
	case "Password:", "Password":
		return []byte(a.password), nil
	default:
		return nil, fmt.Errorf("unknown LOGIN prompt: %s", fromServer)
	}
}
