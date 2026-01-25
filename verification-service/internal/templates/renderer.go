package templates

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"time"
)

//go:embed *.html
var templateFS embed.FS

// Renderer handles email template rendering
type Renderer struct {
	templates map[string]*template.Template
}

// EmailData contains common data for all email templates
type EmailData struct {
	// Common fields
	Subject   string
	Preheader string
	Email     string
	Year      int

	// Verification fields
	Code           string
	ExpiryMinutes  int
	BusinessName   string
	VerificationLink string

	// Welcome pack fields
	FirstName     string
	AdminURL      string
	StorefrontURL string
	DashboardURL  string
}

// NewRenderer creates a new template renderer
func NewRenderer() (*Renderer, error) {
	r := &Renderer{
		templates: make(map[string]*template.Template),
	}

	// Load base template
	baseContent, err := templateFS.ReadFile("base.html")
	if err != nil {
		return nil, fmt.Errorf("failed to read base template: %w", err)
	}

	// Template names to load
	templateNames := []string{
		"email_verification",
		"verification_link",
		"password_reset",
		"welcome_pack",
		"customer_otp",
	}

	for _, name := range templateNames {
		// Read the specific template
		content, err := templateFS.ReadFile(name + ".html")
		if err != nil {
			return nil, fmt.Errorf("failed to read template %s: %w", name, err)
		}

		// Parse base + specific template
		tmpl, err := template.New("email").Parse(string(baseContent))
		if err != nil {
			return nil, fmt.Errorf("failed to parse base template for %s: %w", name, err)
		}

		_, err = tmpl.Parse(string(content))
		if err != nil {
			return nil, fmt.Errorf("failed to parse template %s: %w", name, err)
		}

		r.templates[name] = tmpl
	}

	return r, nil
}

// Render renders a template with the given data
func (r *Renderer) Render(templateName string, data *EmailData) (string, error) {
	tmpl, ok := r.templates[templateName]
	if !ok {
		return "", fmt.Errorf("template %s not found", templateName)
	}

	// Set defaults
	if data.Year == 0 {
		data.Year = time.Now().Year()
	}
	if data.ExpiryMinutes == 0 {
		data.ExpiryMinutes = 10
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template %s: %w", templateName, err)
	}

	return buf.String(), nil
}

// RenderEmailVerification renders the email verification template
func (r *Renderer) RenderEmailVerification(email, code, businessName string, expiryMinutes int) (string, string, error) {
	subject := "Verify Your Email Address"
	if businessName != "" {
		subject = fmt.Sprintf("Verify your email - %s", businessName)
	}

	data := &EmailData{
		Subject:       subject,
		Preheader:     fmt.Sprintf("Your verification code is %s", code),
		Email:         email,
		Code:          code,
		BusinessName:  businessName,
		ExpiryMinutes: expiryMinutes,
	}

	body, err := r.Render("email_verification", data)
	if err != nil {
		return "", "", err
	}

	return subject, body, nil
}

// RenderVerificationLink renders the verification link template
func (r *Renderer) RenderVerificationLink(email, verificationLink, businessName string) (string, string, error) {
	subject := "Verify your email address"
	if businessName != "" {
		subject = fmt.Sprintf("Verify your email - %s", businessName)
	}

	data := &EmailData{
		Subject:          subject,
		Preheader:        "Click to verify your email address",
		Email:            email,
		BusinessName:     businessName,
		VerificationLink: verificationLink,
	}

	body, err := r.Render("verification_link", data)
	if err != nil {
		return "", "", err
	}

	return subject, body, nil
}

// RenderPasswordReset renders the password reset template
func (r *Renderer) RenderPasswordReset(email, code string, expiryMinutes int) (string, string, error) {
	subject := "Reset Your Password"

	data := &EmailData{
		Subject:       subject,
		Preheader:     fmt.Sprintf("Your password reset code is %s", code),
		Email:         email,
		Code:          code,
		ExpiryMinutes: expiryMinutes,
	}

	body, err := r.Render("password_reset", data)
	if err != nil {
		return "", "", err
	}

	return subject, body, nil
}

// RenderWelcomePack renders the welcome pack template
func (r *Renderer) RenderWelcomePack(email, firstName, businessName, adminURL, storefrontURL string) (string, string, error) {
	subject := fmt.Sprintf("Welcome to %s - Your Store is Ready!", businessName)

	data := &EmailData{
		Subject:       subject,
		Preheader:     fmt.Sprintf("Congratulations! Your %s store is ready to go live", businessName),
		Email:         email,
		FirstName:     firstName,
		BusinessName:  businessName,
		AdminURL:      adminURL,
		StorefrontURL: storefrontURL,
	}

	body, err := r.Render("welcome_pack", data)
	if err != nil {
		return "", "", err
	}

	return subject, body, nil
}

// RenderCustomerOTP renders the customer OTP verification template
func (r *Renderer) RenderCustomerOTP(email, code, businessName string, expiryMinutes int) (string, string, error) {
	subject := "Verify your email address"
	if businessName != "" {
		subject = fmt.Sprintf("Verify your email - %s", businessName)
	}

	data := &EmailData{
		Subject:       subject,
		Preheader:     fmt.Sprintf("Your verification code is %s - expires in %d minutes", code, expiryMinutes),
		Email:         email,
		Code:          code,
		BusinessName:  businessName,
		ExpiryMinutes: expiryMinutes,
	}

	body, err := r.Render("customer_otp", data)
	if err != nil {
		return "", "", err
	}

	return subject, body, nil
}

// DefaultRenderer is a package-level renderer instance
var defaultRenderer *Renderer

// Init initializes the default renderer
func Init() error {
	var err error
	defaultRenderer, err = NewRenderer()
	return err
}

// GetRenderer returns the default renderer
func GetRenderer() *Renderer {
	return defaultRenderer
}

// RenderEmailVerificationDefault renders using the default renderer
func RenderEmailVerificationDefault(email, code, businessName string, expiryMinutes int) (string, string, error) {
	if defaultRenderer == nil {
		if err := Init(); err != nil {
			return "", "", err
		}
	}
	return defaultRenderer.RenderEmailVerification(email, code, businessName, expiryMinutes)
}

// RenderVerificationLinkDefault renders using the default renderer
func RenderVerificationLinkDefault(email, verificationLink, businessName string) (string, string, error) {
	if defaultRenderer == nil {
		if err := Init(); err != nil {
			return "", "", err
		}
	}
	return defaultRenderer.RenderVerificationLink(email, verificationLink, businessName)
}

// RenderPasswordResetDefault renders using the default renderer
func RenderPasswordResetDefault(email, code string, expiryMinutes int) (string, string, error) {
	if defaultRenderer == nil {
		if err := Init(); err != nil {
			return "", "", err
		}
	}
	return defaultRenderer.RenderPasswordReset(email, code, expiryMinutes)
}

// RenderWelcomePackDefault renders using the default renderer
func RenderWelcomePackDefault(email, firstName, businessName, adminURL, storefrontURL string) (string, string, error) {
	if defaultRenderer == nil {
		if err := Init(); err != nil {
			return "", "", err
		}
	}
	return defaultRenderer.RenderWelcomePack(email, firstName, businessName, adminURL, storefrontURL)
}

// RenderCustomerOTPDefault renders customer OTP using the default renderer
func RenderCustomerOTPDefault(email, code, businessName string, expiryMinutes int) (string, string, error) {
	if defaultRenderer == nil {
		if err := Init(); err != nil {
			return "", "", err
		}
	}
	return defaultRenderer.RenderCustomerOTP(email, code, businessName, expiryMinutes)
}
