package services

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

// SMTPProvider implements email sending via SMTP
type SMTPProvider struct {
	host     string
	port     string
	username string
	password string
	from     string
	fromName string
}

// NewSMTPProvider creates a new SMTP email provider
func NewSMTPProvider(config *ProviderConfig) *SMTPProvider {
	return &SMTPProvider{
		host:     config.SMTPHost,
		port:     fmt.Sprintf("%d", config.SMTPPort),
		username: config.SMTPUsername,
		password: config.SMTPPassword,
		from:     config.SMTPFrom,
		fromName: "Tesseract Hub",
	}
}

// Send sends an email via SMTP
func (p *SMTPProvider) Send(ctx context.Context, message *Message) (*SendResult, error) {
	// Build email
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

	// Add CC and BCC if provided
	if len(message.CC) > 0 {
		headers["Cc"] = strings.Join(message.CC, ", ")
	}
	if len(message.BCC) > 0 {
		headers["Bcc"] = strings.Join(message.BCC, ", ")
	}

	// Add Reply-To if provided
	if message.ReplyTo != "" {
		headers["Reply-To"] = message.ReplyTo
	}

	// Add custom headers
	for key, value := range message.Headers {
		headers[key] = value
	}

	// Build message body
	var body string
	if message.BodyHTML != "" {
		headers["Content-Type"] = "text/html; charset=utf-8"
		body = message.BodyHTML
	} else {
		headers["Content-Type"] = "text/plain; charset=utf-8"
		body = message.Body
	}

	// Construct email
	var emailBuilder strings.Builder
	for k, v := range headers {
		emailBuilder.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	emailBuilder.WriteString("\r\n")
	emailBuilder.WriteString(body)

	// Collect all recipients
	recipients := []string{message.To}
	recipients = append(recipients, message.CC...)
	recipients = append(recipients, message.BCC...)

	// Send email
	auth := smtp.PlainAuth("", p.username, p.password, p.host)
	addr := net.JoinHostPort(p.host, p.port)

	// Connect with TLS
	tlsConfig := &tls.Config{
		ServerName:         p.host,
		InsecureSkipVerify: false,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		// Try without TLS
		err = smtp.SendMail(addr, auth, p.from, recipients, []byte(emailBuilder.String()))
		if err != nil {
			return &SendResult{
				ProviderName: "SMTP",
				Success:      false,
				Error:        err,
			}, err
		}
	} else {
		defer conn.Close()

		client, err := smtp.NewClient(conn, p.host)
		if err != nil {
			return &SendResult{
				ProviderName: "SMTP",
				Success:      false,
				Error:        err,
			}, err
		}
		defer client.Quit()

		if err = client.Auth(auth); err != nil {
			return &SendResult{
				ProviderName: "SMTP",
				Success:      false,
				Error:        err,
			}, err
		}

		if err = client.Mail(p.from); err != nil {
			return &SendResult{
				ProviderName: "SMTP",
				Success:      false,
				Error:        err,
			}, err
		}

		for _, recipient := range recipients {
			if err = client.Rcpt(recipient); err != nil {
				return &SendResult{
					ProviderName: "SMTP",
					Success:      false,
					Error:        err,
				}, err
			}
		}

		w, err := client.Data()
		if err != nil {
			return &SendResult{
				ProviderName: "SMTP",
				Success:      false,
				Error:        err,
			}, err
		}

		_, err = w.Write([]byte(emailBuilder.String()))
		if err != nil {
			return &SendResult{
				ProviderName: "SMTP",
				Success:      false,
				Error:        err,
			}, err
		}

		err = w.Close()
		if err != nil {
			return &SendResult{
				ProviderName: "SMTP",
				Success:      false,
				Error:        err,
			}, err
		}
	}

	return &SendResult{
		ProviderName: "SMTP",
		Success:      true,
		ProviderData: map[string]interface{}{
			"to":      message.To,
			"subject": message.Subject,
		},
	}, nil
}

// GetName returns the provider name
func (p *SMTPProvider) GetName() string {
	return "SMTP"
}

// SupportsChannel returns the supported channel
func (p *SMTPProvider) SupportsChannel() string {
	return "EMAIL"
}

// SendGridProvider implements email sending via SendGrid
type SendGridProvider struct {
	apiKey   string
	from     string
	fromName string
	client   *sendgrid.Client
}

// NewSendGridProvider creates a new SendGrid email provider
func NewSendGridProvider(config *ProviderConfig) *SendGridProvider {
	return &SendGridProvider{
		apiKey:   config.SendGridAPIKey,
		from:     config.SendGridFrom,
		fromName: "Tesseract Hub",
		client:   sendgrid.NewSendClient(config.SendGridAPIKey),
	}
}

// Send sends an email via SendGrid
func (p *SendGridProvider) Send(ctx context.Context, message *Message) (*SendResult, error) {
	// Build from address
	from := mail.NewEmail(p.fromName, p.from)
	if message.From != "" {
		fromName := message.FromName
		if fromName == "" {
			fromName = message.From
		}
		from = mail.NewEmail(fromName, message.From)
	}

	// Build to address
	to := mail.NewEmail("", message.To)

	// Create message
	var m *mail.SGMailV3
	if message.BodyHTML != "" {
		m = mail.NewSingleEmail(from, message.Subject, to, message.Body, message.BodyHTML)
	} else {
		m = mail.NewSingleEmail(from, message.Subject, to, message.Body, "")
	}

	// Add CC recipients
	if len(message.CC) > 0 {
		p := mail.NewPersonalization()
		p.AddTos(to)
		for _, cc := range message.CC {
			p.AddCCs(mail.NewEmail("", cc))
		}
		m.Personalizations = []*mail.Personalization{p}
	}

	// Add BCC recipients
	if len(message.BCC) > 0 {
		if m.Personalizations == nil || len(m.Personalizations) == 0 {
			p := mail.NewPersonalization()
			p.AddTos(to)
			m.Personalizations = []*mail.Personalization{p}
		}
		for _, bcc := range message.BCC {
			m.Personalizations[0].AddBCCs(mail.NewEmail("", bcc))
		}
	}

	// Add Reply-To
	if message.ReplyTo != "" {
		m.SetReplyTo(mail.NewEmail("", message.ReplyTo))
	}

	// Add custom headers
	if len(message.Headers) > 0 {
		m.Headers = message.Headers
	}

	// Add attachments
	if len(message.Attachments) > 0 {
		for _, att := range message.Attachments {
			a := mail.NewAttachment()
			a.SetFilename(att.Filename)
			a.SetType(att.ContentType)
			a.SetContent(string(att.Content))
			m.AddAttachment(a)
		}
	}

	// Disable click tracking for transactional emails
	// This prevents SendGrid from rewriting URLs (which causes SSL issues with tracking domain)
	trackingSettings := mail.NewTrackingSettings()
	clickTracking := mail.NewClickTrackingSetting()
	clickTracking.SetEnable(false)
	clickTracking.SetEnableText(false)
	trackingSettings.SetClickTracking(clickTracking)
	// Also disable open tracking for privacy
	openTracking := mail.NewOpenTrackingSetting()
	openTracking.SetEnable(false)
	trackingSettings.SetOpenTracking(openTracking)
	m.SetTrackingSettings(trackingSettings)

	// Send the email
	response, err := p.client.Send(m)
	if err != nil {
		return &SendResult{
			ProviderName: "SendGrid",
			Success:      false,
			Error:        err,
		}, err
	}

	// Check response status
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		// Get X-Message-Id from headers
		var messageID string
		if ids, ok := response.Headers["X-Message-Id"]; ok && len(ids) > 0 {
			messageID = ids[0]
		}
		return &SendResult{
			ProviderID:   messageID,
			ProviderName: "SendGrid",
			Success:      true,
			ProviderData: map[string]interface{}{
				"status_code": response.StatusCode,
				"to":          message.To,
				"subject":     message.Subject,
			},
		}, nil
	}

	// Request failed
	return &SendResult{
		ProviderName: "SendGrid",
		Success:      false,
		Error:        fmt.Errorf("SendGrid API error: %d - %s", response.StatusCode, response.Body),
	}, fmt.Errorf("SendGrid API error: %d", response.StatusCode)
}

// GetName returns the provider name
func (p *SendGridProvider) GetName() string {
	return "SendGrid"
}

// SupportsChannel returns the supported channel
func (p *SendGridProvider) SupportsChannel() string {
	return "EMAIL"
}
