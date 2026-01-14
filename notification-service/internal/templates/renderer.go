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

// Address represents a shipping/billing address
type Address struct {
	Name       string
	Line1      string
	Line2      string
	City       string
	State      string
	PostalCode string
	Country    string
}

// OrderItem represents an item in an order
type OrderItem struct {
	Name     string
	SKU      string
	ImageURL string
	Quantity int
	Price    string
	Currency string
}

// ProductStock represents a product with stock info
type ProductStock struct {
	Name       string
	SKU        string
	ImageURL   string
	StockLevel int
}

// EmailData contains data for all email templates
type EmailData struct {
	// Common fields
	Subject         string
	Preheader       string
	Email           string
	Year            int
	BusinessName    string
	BusinessInitial string
	SupportEmail    string

	// Order fields
	OrderNumber     string
	OrderDate       string
	Currency        string
	Items           []OrderItem
	Subtotal        string
	Discount        string
	Shipping        string
	Tax             string
	Total           string
	ShippingAddress *Address
	PaymentMethod   string
	TrackingURL     string
	OrderDetailsURL string

	// Shipping fields
	Carrier           string
	TrackingNumber    string
	EstimatedDelivery string
	DeliveryDate      string
	DeliveryLocation  string

	// Cancellation fields
	CancelledDate      string
	CancellationReason string
	RefundAmount       string
	RefundDays         string
	ShopURL            string

	// Payment fields
	TransactionID string
	Amount        string
	PaymentDate   string
	FailureReason string
	RetryURL      string
	PaymentStatus string // CAPTURED, FAILED, REFUNDED for unified payment template

	// Customer fields
	CustomerName string
	WelcomeOffer string
	PromoCode    string
	ReviewURL    string

	// Inventory fields
	Products              []ProductStock
	TotalLowStockItems    int
	TotalOutOfStockItems  int
	TotalAffectedProducts int
	InventoryURL          string

	// Review fields
	ReviewID      string
	ProductName   string
	ProductSKU    string
	ReviewTitle   string
	ReviewContent string
	Rating        int
	MaxRating     int
	RatingStars   template.HTML // Pre-formatted star display (not escaped)
	ReviewDate    string
	ReviewStatus  string
	RejectReason  string
	ModeratedBy   string
	IsVerified    bool
	ReviewsURL    string
	ProductURL    string

	// Ticket fields
	TicketID         string
	TicketNumber     string
	TicketSubject    string
	TicketCategory   string
	TicketPriority   string
	TicketStatus     string
	Description      string
	AssignedTo       string
	AssignedToName   string
	Resolution       string
	TicketURL        string
	Reason           string // For on-hold, cancelled statuses
	EscalatedBy      string
	EscalationReason string
	ReopenedBy       string
	OldStatus        string
	NewStatus        string

	// Vendor fields
	VendorID           string
	VendorName         string
	VendorEmail        string
	VendorBusinessName string
	VendorStatus       string
	PreviousStatus     string
	StatusReason       string
	ReviewedBy         string
	VendorURL          string

	// Coupon fields
	CouponID       string
	CouponCode     string
	DiscountType   string
	DiscountValue  string
	DiscountAmount string
	OrderValue     string
	ValidFrom      string
	ValidUntil     string
	CouponStatus   string
	CouponsURL     string

	// Auth/Security fields
	ResetCode          string
	ResetURL           string
	ResetPasswordURL   string
	VerificationLink   string
	VerificationToken  string
	VerificationExpiry string

	// Login notification fields
	LoginTime     string
	LoginLocation string
	IPAddress     string
	DeviceInfo    string
	UserAgent     string
	LoginMethod   string

	// Tenant onboarding fields
	SessionID      string
	TenantSlug     string
	Product        string
	AdminHost      string
	StorefrontHost string
	BaseDomain     string
	AdminURL       string
	StorefrontURL  string

	// Order status for unified template
	OrderStatus string

	// Staff invitation fields
	StaffName      string
	StaffEmail     string
	InviterName    string
	Role           string
	ActivationLink string

	// Approval workflow fields
	ApprovalID         string
	ApprovalStatus     string // PENDING, APPROVED, REJECTED, CANCELLED, EXPIRED, ESCALATED
	ApprovalPriority   string // low, normal, high, urgent
	ActionType         string // The type of action needing approval (refund, cancel_order, etc.)
	ActionTypeDisplay  string // Human-readable action type
	ResourceType       string // order, product, payment, etc.
	ResourceID         string
	RequesterID        string
	RequesterName      string
	RequesterEmail     string
	ApproverID         string
	ApproverName       string
	ApproverEmail      string
	ApproverRole       string
	ApprovalReason     string // Reason provided by requester
	ApprovalComment    string // Comment by approver when approving/rejecting
	ApprovalExpiresAt  string
	ApprovalCreatedAt  string
	ApprovalDecidedAt  string
	ApprovalURL        string // URL to view/act on the approval
	EscalatedFromID    string
	EscalatedFromName  string
	EscalationLevel    int
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
		// Order templates (unified dynamic template)
		"order_customer",
		// Payment templates (unified dynamic template)
		"payment_customer",
		// Customer templates
		"customer_welcome",
		// Inventory templates
		"low_stock_alert",
		// Review templates (unified dynamic template + admin template)
		"review_customer",
		"review_submitted_admin",
		// Ticket templates (unified dynamic templates)
		"ticket_customer",
		"ticket_admin",
		// Vendor templates (unified dynamic template + admin template)
		"vendor_customer",
		"vendor_application",
		// Coupon templates
		"coupon_applied",
		"coupon_created",
		"coupon_expired",
		// Auth templates
		"password_reset",
		// Customer verification
		"customer_verification",
		// Login notification
		"login_notification",
		// Tenant onboarding templates
		"tenant_welcome_pack",
		"verification_link",
		// Approval workflow templates
		"approval_approver",
		"approval_requester",
		// Staff templates
		"staff_invitation",
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
	if data.Currency == "" {
		data.Currency = "$"
	}
	if data.SupportEmail == "" {
		data.SupportEmail = "support@tesserix.app"
	}
	if data.BusinessInitial == "" && data.BusinessName != "" && len(data.BusinessName) > 0 {
		data.BusinessInitial = string(data.BusinessName[0])
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template %s: %w", templateName, err)
	}

	return buf.String(), nil
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

// GetDefaultRenderer returns or initializes the default renderer
func GetDefaultRenderer() (*Renderer, error) {
	if defaultRenderer == nil {
		if err := Init(); err != nil {
			return nil, err
		}
	}
	return defaultRenderer, nil
}
