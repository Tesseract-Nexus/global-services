package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// StorefrontThemeSettings represents the storefront theme configuration
// This model stores storefront-specific theme settings
// Each tenant (vendor) can have multiple storefronts, each with its own theme
type StorefrontThemeSettings struct {
	ID                 uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID           uuid.UUID      `json:"tenantId" gorm:"type:uuid;not null;index"`
	StorefrontID       uuid.UUID      `json:"storefrontId" gorm:"type:uuid;not null;uniqueIndex:idx_storefront_theme_unique"`
	ThemeTemplate      string         `json:"themeTemplate" gorm:"type:varchar(50);not null;default:'vibrant'"`
	PrimaryColor       string         `json:"primaryColor" gorm:"type:varchar(20);not null;default:'#8B5CF6'"`
	SecondaryColor     string         `json:"secondaryColor" gorm:"type:varchar(20);not null;default:'#EC4899'"`
	AccentColor        *string        `json:"accentColor,omitempty" gorm:"type:varchar(20)"`
	LogoURL            *string        `json:"logoUrl,omitempty" gorm:"type:text"`
	FaviconURL         *string        `json:"faviconUrl,omitempty" gorm:"type:text"`
	FontPrimary        string         `json:"fontPrimary" gorm:"type:varchar(100);default:'Inter'"`
	FontSecondary      string         `json:"fontSecondary" gorm:"type:varchar(100);default:'system-ui'"`
	ColorMode          string         `json:"colorMode" gorm:"type:varchar(20);default:'both'"` // light, dark, both, system
	HeaderConfig       datatypes.JSON `json:"headerConfig" gorm:"type:jsonb;default:'{}'"`
	HomepageConfig     datatypes.JSON `json:"homepageConfig" gorm:"type:jsonb;default:'{}'"`
	FooterConfig       datatypes.JSON `json:"footerConfig" gorm:"type:jsonb;default:'{}'"`
	ProductConfig      datatypes.JSON `json:"productConfig" gorm:"type:jsonb;default:'{}'"`
	CheckoutConfig     datatypes.JSON `json:"checkoutConfig" gorm:"type:jsonb;default:'{}'"`
	CustomCSS          *string        `json:"customCss,omitempty" gorm:"type:text"`
	// Enhanced configuration options
	TypographyConfig   datatypes.JSON `json:"typographyConfig,omitempty" gorm:"type:jsonb;default:'{}'"`
	LayoutConfig       datatypes.JSON `json:"layoutConfig,omitempty" gorm:"type:jsonb;default:'{}'"`
	SpacingStyleConfig datatypes.JSON `json:"spacingStyleConfig,omitempty" gorm:"type:jsonb;default:'{}'"`
	MobileConfig       datatypes.JSON `json:"mobileConfig,omitempty" gorm:"type:jsonb;default:'{}'"`
	AdvancedConfig     datatypes.JSON `json:"advancedConfig,omitempty" gorm:"type:jsonb;default:'{}'"`
	// Content pages (Privacy Policy, Terms, FAQ, etc.)
	ContentPages       datatypes.JSON `json:"contentPages,omitempty" gorm:"type:jsonb;default:'[]'"`
	Version            int            `json:"version" gorm:"default:1"`
	CreatedAt          time.Time      `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt          time.Time      `json:"updatedAt" gorm:"autoUpdateTime"`
	DeletedAt          gorm.DeletedAt `json:"deletedAt,omitempty" gorm:"index"`
	CreatedBy          *uuid.UUID     `json:"createdBy,omitempty" gorm:"type:uuid"`
	UpdatedBy          *uuid.UUID     `json:"updatedBy,omitempty" gorm:"type:uuid"`
}


// HeaderConfigData represents the header configuration structure
type HeaderConfigData struct {
	ShowAnnouncement bool                `json:"showAnnouncement"`
	AnnouncementText string              `json:"announcementText,omitempty"`
	AnnouncementLink *string             `json:"announcementLink,omitempty"`
	NavLinks         []StorefrontNavLink `json:"navLinks,omitempty"`
	ShowSearch       bool                `json:"showSearch"`
	ShowCart         bool                `json:"showCart"`
	ShowAccount      bool                `json:"showAccount"`
	StickyHeader     bool                `json:"stickyHeader"`
}

// HomepageConfigData represents the homepage configuration structure
type HomepageConfigData struct {
	ShowHero           bool                 `json:"showHero"`
	HeroTitle          string               `json:"heroTitle,omitempty"`
	HeroSubtitle       string               `json:"heroSubtitle,omitempty"`
	HeroImage          *string              `json:"heroImage,omitempty"`
	HeroCtaText        string               `json:"heroCtaText,omitempty"`
	HeroCtaLink        string               `json:"heroCtaLink,omitempty"`
	Sections           []StorefrontSection  `json:"sections,omitempty"`
	FeaturedProductIDs []string             `json:"featuredProductIds,omitempty"`
	ShowNewsletter     bool                 `json:"showNewsletter"`
	NewsletterTitle    string               `json:"newsletterTitle,omitempty"`
	NewsletterSubtitle string               `json:"newsletterSubtitle,omitempty"`
}

// FooterConfigData represents the footer configuration structure
type FooterConfigData struct {
	ShowFooter      bool                  `json:"showFooter"`
	CopyrightText   string                `json:"copyrightText,omitempty"`
	LinkGroups      []FooterLinkGroup     `json:"linkGroups,omitempty"`
	SocialLinks     []StorefrontSocialLink `json:"socialLinks,omitempty"`
	ShowNewsletter  bool                  `json:"showNewsletter"`
	NewsletterTitle string                `json:"newsletterTitle,omitempty"`
	ContactEmail    *string               `json:"contactEmail,omitempty"`
	ContactPhone    *string               `json:"contactPhone,omitempty"`
	ContactAddress  *string               `json:"contactAddress,omitempty"`
	ShowPoweredBy   bool                  `json:"showPoweredBy"`
}

// ProductConfigData represents the product display configuration
type ProductConfigData struct {
	GridColumns        int    `json:"gridColumns"`
	CardStyle          string `json:"cardStyle"`
	ShowQuickView      bool   `json:"showQuickView"`
	ShowWishlistButton bool   `json:"showWishlistButton"`
	ShowRatings        bool   `json:"showRatings"`
	ShowReviewCount    bool   `json:"showReviewCount"`
	ImageAspectRatio   string `json:"imageAspectRatio"`
	HoverEffect        string `json:"hoverEffect"`
}

// CheckoutConfigData represents the checkout configuration
type CheckoutConfigData struct {
	GuestCheckoutEnabled      bool `json:"guestCheckoutEnabled"`
	AccountCreationAtCheckout bool `json:"accountCreationAtCheckout"`
	RequirePhone              bool `json:"requirePhone"`
	RequireCompany            bool `json:"requireCompany"`
	ShowOrderNotes            bool `json:"showOrderNotes"`
	ShowGiftOptions           bool `json:"showGiftOptions"`
	ShowTrustBadges           bool `json:"showTrustBadges"`
	TermsRequired             bool `json:"termsRequired"`
	TermsLink                 *string `json:"termsLink,omitempty"`
	PrivacyLink               *string `json:"privacyLink,omitempty"`
}

// StorefrontNavLink represents a navigation link
type StorefrontNavLink struct {
	Label    string `json:"label"`
	Href     string `json:"href"`
	External bool   `json:"external,omitempty"`
	Position int    `json:"position,omitempty"`
}

// StorefrontSection represents a homepage section
type StorefrontSection struct {
	Type     string                 `json:"type"`
	Enabled  bool                   `json:"enabled"`
	Position int                    `json:"position"`
	Config   map[string]interface{} `json:"config,omitempty"`
}

// FooterLinkGroup represents a group of footer links
type FooterLinkGroup struct {
	Title string              `json:"title"`
	Links []StorefrontNavLink `json:"links"`
}

// StorefrontSocialLink represents a social media link
type StorefrontSocialLink struct {
	Platform string `json:"platform"`
	URL      string `json:"url"`
}

// CreateStorefrontThemeRequest represents a request to create storefront theme settings
type CreateStorefrontThemeRequest struct {
	StorefrontID       string                 `json:"storefrontId,omitempty"` // Optional - can also be provided via URL path
	ThemeTemplate      string                 `json:"themeTemplate"`
	PrimaryColor       string                 `json:"primaryColor"`
	SecondaryColor     string                 `json:"secondaryColor"`
	AccentColor        *string                `json:"accentColor,omitempty"`
	LogoURL            *string                `json:"logoUrl,omitempty"`
	FaviconURL         *string                `json:"faviconUrl,omitempty"`
	FontPrimary        string                 `json:"fontPrimary,omitempty"`
	FontSecondary      string                 `json:"fontSecondary,omitempty"`
	ColorMode          string                 `json:"colorMode,omitempty"` // light, dark, both, system
	HeaderConfig       map[string]interface{} `json:"headerConfig,omitempty"`
	HomepageConfig     map[string]interface{} `json:"homepageConfig,omitempty"`
	FooterConfig       map[string]interface{} `json:"footerConfig,omitempty"`
	ProductConfig      map[string]interface{} `json:"productConfig,omitempty"`
	CheckoutConfig     map[string]interface{} `json:"checkoutConfig,omitempty"`
	CustomCSS          *string                `json:"customCss,omitempty"`
	// Enhanced configuration options
	TypographyConfig   map[string]interface{} `json:"typographyConfig,omitempty"`
	LayoutConfig       map[string]interface{} `json:"layoutConfig,omitempty"`
	SpacingStyleConfig map[string]interface{} `json:"spacingStyleConfig,omitempty"`
	MobileConfig       map[string]interface{} `json:"mobileConfig,omitempty"`
	AdvancedConfig     map[string]interface{} `json:"advancedConfig,omitempty"`
}

// UpdateStorefrontThemeRequest represents a request to update storefront theme settings
type UpdateStorefrontThemeRequest struct {
	ThemeTemplate      *string                `json:"themeTemplate,omitempty"`
	PrimaryColor       *string                `json:"primaryColor,omitempty"`
	SecondaryColor     *string                `json:"secondaryColor,omitempty"`
	AccentColor        *string                `json:"accentColor,omitempty"`
	LogoURL            *string                `json:"logoUrl,omitempty"`
	FaviconURL         *string                `json:"faviconUrl,omitempty"`
	FontPrimary        *string                `json:"fontPrimary,omitempty"`
	FontSecondary      *string                `json:"fontSecondary,omitempty"`
	ColorMode          *string                `json:"colorMode,omitempty"` // light, dark, both, system
	HeaderConfig       map[string]interface{} `json:"headerConfig,omitempty"`
	HomepageConfig     map[string]interface{} `json:"homepageConfig,omitempty"`
	FooterConfig       map[string]interface{} `json:"footerConfig,omitempty"`
	ProductConfig      map[string]interface{} `json:"productConfig,omitempty"`
	CheckoutConfig     map[string]interface{} `json:"checkoutConfig,omitempty"`
	CustomCSS          *string                `json:"customCss,omitempty"`
	// Enhanced configuration options
	TypographyConfig   map[string]interface{} `json:"typographyConfig,omitempty"`
	LayoutConfig       map[string]interface{} `json:"layoutConfig,omitempty"`
	SpacingStyleConfig map[string]interface{} `json:"spacingStyleConfig,omitempty"`
	MobileConfig       map[string]interface{} `json:"mobileConfig,omitempty"`
	AdvancedConfig     map[string]interface{} `json:"advancedConfig,omitempty"`
	// Content pages (Privacy Policy, Terms, FAQ, etc.)
	ContentPages       []ContentPage          `json:"contentPages,omitempty"`
}

// StorefrontThemeResponse represents the API response for storefront theme
type StorefrontThemeResponse struct {
	Success bool                     `json:"success"`
	Data    *StorefrontThemeSettings `json:"data,omitempty"`
	Message string                   `json:"message,omitempty"`
}

// StorefrontThemePreset represents a theme preset
type StorefrontThemePreset struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	PrimaryColor   string `json:"primaryColor"`
	SecondaryColor string `json:"secondaryColor"`
	AccentColor    string `json:"accentColor,omitempty"`
	BackgroundColor string `json:"backgroundColor"`
	TextColor      string `json:"textColor"`
	Preview        string `json:"preview,omitempty"`
}

// Default theme presets - Core themes
var DefaultThemePresets = []StorefrontThemePreset{
	// ========================================
	// Core Themes (Original 12)
	// ========================================
	{
		ID:              "vibrant",
		Name:            "Vibrant",
		Description:     "Bold and energetic with purple and pink hues",
		PrimaryColor:    "#8B5CF6",
		SecondaryColor:  "#EC4899",
		AccentColor:     "#F59E0B",
		BackgroundColor: "#FAFAF9",
		TextColor:       "#1F2937",
	},
	{
		ID:              "minimal",
		Name:            "Minimal",
		Description:     "Clean and simple with neutral colors",
		PrimaryColor:    "#171717",
		SecondaryColor:  "#737373",
		AccentColor:     "#3B82F6",
		BackgroundColor: "#FFFFFF",
		TextColor:       "#171717",
	},
	{
		ID:              "dark",
		Name:            "Dark",
		Description:     "Modern dark theme with high contrast",
		PrimaryColor:    "#818CF8",
		SecondaryColor:  "#A78BFA",
		AccentColor:     "#34D399",
		BackgroundColor: "#111827",
		TextColor:       "#F9FAFB",
	},
	{
		ID:              "neon",
		Name:            "Neon",
		Description:     "Futuristic with bright neon accents",
		PrimaryColor:    "#22D3EE",
		SecondaryColor:  "#A855F7",
		AccentColor:     "#F472B6",
		BackgroundColor: "#0F172A",
		TextColor:       "#E2E8F0",
	},
	{
		ID:              "ocean",
		Name:            "Ocean",
		Description:     "Calm and serene with blue tones",
		PrimaryColor:    "#0EA5E9",
		SecondaryColor:  "#06B6D4",
		AccentColor:     "#14B8A6",
		BackgroundColor: "#F0F9FF",
		TextColor:       "#0C4A6E",
	},
	{
		ID:              "sunset",
		Name:            "Sunset",
		Description:     "Warm and inviting with orange and red tones",
		PrimaryColor:    "#F97316",
		SecondaryColor:  "#EF4444",
		AccentColor:     "#FBBF24",
		BackgroundColor: "#FFFBEB",
		TextColor:       "#7C2D12",
	},
	{
		ID:              "forest",
		Name:            "Forest",
		Description:     "Natural and organic with green tones",
		PrimaryColor:    "#16A34A",
		SecondaryColor:  "#84CC16",
		AccentColor:     "#CA8A04",
		BackgroundColor: "#F0FDF4",
		TextColor:       "#14532D",
	},
	{
		ID:              "luxury",
		Name:            "Luxury",
		Description:     "Sophisticated with gold and dark accents",
		PrimaryColor:    "#B8860B",
		SecondaryColor:  "#1C1C1C",
		AccentColor:     "#D4AF37",
		BackgroundColor: "#1C1C1C",
		TextColor:       "#F5F5F5",
	},
	{
		ID:              "rose",
		Name:            "Rose",
		Description:     "Soft and romantic with pink tones",
		PrimaryColor:    "#EC4899",
		SecondaryColor:  "#F472B6",
		AccentColor:     "#FB7185",
		BackgroundColor: "#FFF1F2",
		TextColor:       "#831843",
	},
	{
		ID:              "corporate",
		Name:            "Corporate",
		Description:     "Professional and trustworthy",
		PrimaryColor:    "#2563EB",
		SecondaryColor:  "#1E40AF",
		AccentColor:     "#0EA5E9",
		BackgroundColor: "#FFFFFF",
		TextColor:       "#1E3A5F",
	},
	{
		ID:              "earthy",
		Name:            "Earthy",
		Description:     "Warm and grounded with natural tones",
		PrimaryColor:    "#92400E",
		SecondaryColor:  "#A16207",
		AccentColor:     "#B45309",
		BackgroundColor: "#FFFBEB",
		TextColor:       "#451A03",
	},
	{
		ID:              "arctic",
		Name:            "Arctic",
		Description:     "Cool and crisp with icy tones",
		PrimaryColor:    "#0891B2",
		SecondaryColor:  "#06B6D4",
		AccentColor:     "#22D3EE",
		BackgroundColor: "#ECFEFF",
		TextColor:       "#164E63",
	},

	// ========================================
	// Industry-Specific Themes (12 new)
	// ========================================
	{
		ID:              "fashion",
		Name:            "Fashion",
		Description:     "Elegant editorial style for apparel and fashion brands",
		PrimaryColor:    "#1A1A2E",
		SecondaryColor:  "#E94560",
		AccentColor:     "#F5F5F5",
		BackgroundColor: "#FFFFFF",
		TextColor:       "#1A1A2E",
	},
	{
		ID:              "streetwear",
		Name:            "Streetwear",
		Description:     "Bold urban aesthetic for street fashion",
		PrimaryColor:    "#0D0D0D",
		SecondaryColor:  "#FF6B35",
		AccentColor:     "#FFBA08",
		BackgroundColor: "#0D0D0D",
		TextColor:       "#FFFFFF",
	},
	{
		ID:              "food",
		Name:            "Fresh Food",
		Description:     "Appetizing colors for groceries and fresh produce",
		PrimaryColor:    "#2D5016",
		SecondaryColor:  "#F4A261",
		AccentColor:     "#E76F51",
		BackgroundColor: "#FEFCE8",
		TextColor:       "#1F2937",
	},
	{
		ID:              "bakery",
		Name:            "Bakery",
		Description:     "Warm cozy vibes for bakeries and pastry shops",
		PrimaryColor:    "#8B4513",
		SecondaryColor:  "#F5DEB3",
		AccentColor:     "#D4A574",
		BackgroundColor: "#FFF8F0",
		TextColor:       "#3D2914",
	},
	{
		ID:              "cafe",
		Name:            "Coffee Shop",
		Description:     "Rich rustic tones for cafes and coffee shops",
		PrimaryColor:    "#3C2415",
		SecondaryColor:  "#D4A574",
		AccentColor:     "#C8A27A",
		BackgroundColor: "#FAF7F4",
		TextColor:       "#2D1B0E",
	},
	{
		ID:              "electronics",
		Name:            "Electronics",
		Description:     "Modern tech aesthetic for gadgets and electronics",
		PrimaryColor:    "#0F1419",
		SecondaryColor:  "#00D4FF",
		AccentColor:     "#7C3AED",
		BackgroundColor: "#0F1419",
		TextColor:       "#E5E7EB",
	},
	{
		ID:              "beauty",
		Name:            "Beauty",
		Description:     "Glamorous soft tones for cosmetics and skincare",
		PrimaryColor:    "#2D1B4E",
		SecondaryColor:  "#E8B4B8",
		AccentColor:     "#D4A5A5",
		BackgroundColor: "#FDF8F8",
		TextColor:       "#2D1B4E",
	},
	{
		ID:              "wellness",
		Name:            "Wellness",
		Description:     "Calming natural tones for health and wellness",
		PrimaryColor:    "#1B4D3E",
		SecondaryColor:  "#A8E6CF",
		AccentColor:     "#88D8B0",
		BackgroundColor: "#F0FFF4",
		TextColor:       "#1B4D3E",
	},
	{
		ID:              "jewelry",
		Name:            "Jewelry",
		Description:     "Premium luxurious feel for jewelry and accessories",
		PrimaryColor:    "#1C1C1C",
		SecondaryColor:  "#D4AF37",
		AccentColor:     "#C9B037",
		BackgroundColor: "#FFFEF7",
		TextColor:       "#1C1C1C",
	},
	{
		ID:              "kids",
		Name:            "Kids & Toys",
		Description:     "Playful colorful design for children products",
		PrimaryColor:    "#FF6B6B",
		SecondaryColor:  "#4ECDC4",
		AccentColor:     "#FFE66D",
		BackgroundColor: "#FFFEF7",
		TextColor:       "#2D3436",
	},
	{
		ID:              "sports",
		Name:            "Sports",
		Description:     "Dynamic energetic style for sports and outdoors",
		PrimaryColor:    "#1E3A5F",
		SecondaryColor:  "#00D9FF",
		AccentColor:     "#FF6B35",
		BackgroundColor: "#F8FAFC",
		TextColor:       "#1E3A5F",
	},
	{
		ID:              "home",
		Name:            "Home & Decor",
		Description:     "Sophisticated modern style for home and furniture",
		PrimaryColor:    "#2C3E50",
		SecondaryColor:  "#E67E22",
		AccentColor:     "#3498DB",
		BackgroundColor: "#FDFBF7",
		TextColor:       "#2C3E50",
	},
}

// GetDefaultHeaderConfig returns the default header configuration
func GetDefaultHeaderConfig() HeaderConfigData {
	return HeaderConfigData{
		ShowAnnouncement: false,
		AnnouncementText: "",
		NavLinks: []StorefrontNavLink{
			{Label: "Products", Href: "/products", Position: 1},
			{Label: "Categories", Href: "/categories", Position: 2},
		},
		ShowSearch:   true,
		ShowCart:     true,
		ShowAccount:  true,
		StickyHeader: true,
	}
}

// GetDefaultHomepageConfig returns the default homepage configuration
func GetDefaultHomepageConfig() HomepageConfigData {
	return HomepageConfigData{
		ShowHero:       true,
		HeroTitle:      "Welcome to Our Store",
		HeroSubtitle:   "Discover amazing products at great prices",
		HeroCtaText:    "Shop Now",
		HeroCtaLink:    "/products",
		Sections:       []StorefrontSection{},
		ShowNewsletter: true,
		NewsletterTitle: "Stay Updated",
		NewsletterSubtitle: "Subscribe to our newsletter for the latest updates",
	}
}

// GetDefaultFooterConfig returns the default footer configuration
func GetDefaultFooterConfig() FooterConfigData {
	return FooterConfigData{
		ShowFooter:     true,
		CopyrightText:  "All rights reserved",
		LinkGroups:     []FooterLinkGroup{},
		SocialLinks:    []StorefrontSocialLink{},
		ShowNewsletter: false,
		ShowPoweredBy:  true,
	}
}

// GetDefaultProductConfig returns the default product configuration
func GetDefaultProductConfig() ProductConfigData {
	return ProductConfigData{
		GridColumns:        4,
		CardStyle:          "default",
		ShowQuickView:      true,
		ShowWishlistButton: true,
		ShowRatings:        true,
		ShowReviewCount:    true,
		ImageAspectRatio:   "square",
		HoverEffect:        "zoom",
	}
}

// GetDefaultCheckoutConfig returns the default checkout configuration
func GetDefaultCheckoutConfig() CheckoutConfigData {
	return CheckoutConfigData{
		GuestCheckoutEnabled:      true,
		AccountCreationAtCheckout: true,
		RequirePhone:              true,
		RequireCompany:            false,
		ShowOrderNotes:            true,
		ShowGiftOptions:           false,
		ShowTrustBadges:           true,
		TermsRequired:             true,
	}
}

// ContentPage represents a content page (Privacy Policy, Terms, FAQ, etc.)
type ContentPage struct {
	ID              string  `json:"id"`
	Type            string  `json:"type"`   // STATIC, POLICY, FAQ, BLOG, LANDING, CUSTOM
	Status          string  `json:"status"` // DRAFT, PUBLISHED, ARCHIVED
	Title           string  `json:"title"`
	Slug            string  `json:"slug"`
	Excerpt         string  `json:"excerpt,omitempty"`
	Content         string  `json:"content"`
	MetaTitle       string  `json:"metaTitle,omitempty"`
	MetaDescription string  `json:"metaDescription,omitempty"`
	ShowInMenu      bool    `json:"showInMenu"`
	ShowInFooter    bool    `json:"showInFooter"`
	IsFeatured      bool    `json:"isFeatured"`
	ViewCount       int     `json:"viewCount"`
	CreatedAt       string  `json:"createdAt"`
	UpdatedAt       string  `json:"updatedAt"`
}

// GetDefaultContentPages returns the default content pages for a new storefront
func GetDefaultContentPages() []ContentPage {
	now := time.Now().Format(time.RFC3339)
	return []ContentPage{
		{
			ID:              "page-privacy-policy",
			Type:            "POLICY",
			Status:          "PUBLISHED",
			Title:           "Privacy Policy",
			Slug:            "privacy-policy",
			Excerpt:         "Learn how we collect, use, and protect your personal information.",
			Content:         `<h2>Privacy Policy</h2>
<p>Your privacy is important to us. This Privacy Policy explains how we collect, use, disclose, and safeguard your information when you visit our website.</p>

<h3>Information We Collect</h3>
<p>We collect information that you provide directly to us, such as when you create an account, make a purchase, or contact us for support.</p>

<h3>How We Use Your Information</h3>
<p>We use the information we collect to:</p>
<ul>
<li>Process your orders and transactions</li>
<li>Send you order confirmations and updates</li>
<li>Respond to your comments and questions</li>
<li>Improve our website and services</li>
</ul>

<h3>Information Sharing</h3>
<p>We do not sell, trade, or otherwise transfer your personal information to third parties without your consent, except as described in this policy.</p>

<h3>Contact Us</h3>
<p>If you have questions about this Privacy Policy, please contact us.</p>`,
			MetaTitle:       "Privacy Policy",
			MetaDescription: "Read our privacy policy to understand how we handle your data.",
			ShowInMenu:      false,
			ShowInFooter:    true,
			IsFeatured:      false,
			ViewCount:       0,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		{
			ID:              "page-terms-of-service",
			Type:            "POLICY",
			Status:          "PUBLISHED",
			Title:           "Terms of Service",
			Slug:            "terms-of-service",
			Excerpt:         "Please read these terms carefully before using our services.",
			Content:         `<h2>Terms of Service</h2>
<p>By accessing and using this website, you accept and agree to be bound by the terms and provisions of this agreement.</p>

<h3>Use of Website</h3>
<p>You may use our website for lawful purposes only. You agree not to use the site in any way that violates any applicable laws or regulations.</p>

<h3>Account Responsibilities</h3>
<p>You are responsible for maintaining the confidentiality of your account and password. You agree to accept responsibility for all activities that occur under your account.</p>

<h3>Products and Services</h3>
<p>We reserve the right to refuse service to anyone for any reason at any time. Prices for our products are subject to change without notice.</p>

<h3>Limitation of Liability</h3>
<p>We shall not be liable for any indirect, incidental, special, consequential, or punitive damages resulting from your use of or inability to use our services.</p>

<h3>Changes to Terms</h3>
<p>We reserve the right to update or modify these terms at any time without prior notice.</p>`,
			MetaTitle:       "Terms of Service",
			MetaDescription: "Read our terms of service to understand your rights and responsibilities.",
			ShowInMenu:      false,
			ShowInFooter:    true,
			IsFeatured:      false,
			ViewCount:       0,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		{
			ID:              "page-refund-policy",
			Type:            "POLICY",
			Status:          "PUBLISHED",
			Title:           "Refund Policy",
			Slug:            "refund-policy",
			Excerpt:         "Learn about our return and refund policies.",
			Content:         `<h2>Refund Policy</h2>
<p>We want you to be completely satisfied with your purchase. If you're not happy with your order, we're here to help.</p>

<h3>Return Window</h3>
<p>You have 30 days from the date of delivery to return most items for a full refund.</p>

<h3>Eligibility</h3>
<p>To be eligible for a return, items must be:</p>
<ul>
<li>In their original condition</li>
<li>Unworn, unwashed, and with tags attached</li>
<li>In the original packaging</li>
</ul>

<h3>How to Return</h3>
<p>To initiate a return, please contact our customer service team. We'll provide you with a return shipping label and instructions.</p>

<h3>Refund Processing</h3>
<p>Once we receive your return, we'll inspect the item and process your refund within 5-7 business days. Refunds will be credited to your original payment method.</p>

<h3>Exchanges</h3>
<p>If you'd like to exchange an item for a different size or color, please contact us and we'll be happy to assist.</p>`,
			MetaTitle:       "Refund Policy",
			MetaDescription: "Learn about our return and refund policies for a worry-free shopping experience.",
			ShowInMenu:      false,
			ShowInFooter:    true,
			IsFeatured:      false,
			ViewCount:       0,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		{
			ID:              "page-shipping-info",
			Type:            "STATIC",
			Status:          "PUBLISHED",
			Title:           "Shipping Information",
			Slug:            "shipping",
			Excerpt:         "Everything you need to know about shipping and delivery.",
			Content:         `<h2>Shipping Information</h2>
<p>We offer fast and reliable shipping to get your order to you as quickly as possible.</p>

<h3>Processing Time</h3>
<p>Orders are typically processed within 1-2 business days. You'll receive a confirmation email with tracking information once your order ships.</p>

<h3>Shipping Options</h3>
<ul>
<li><strong>Standard Shipping:</strong> 5-7 business days</li>
<li><strong>Express Shipping:</strong> 2-3 business days</li>
<li><strong>Next Day Delivery:</strong> 1 business day (order by 2 PM)</li>
</ul>

<h3>Shipping Rates</h3>
<p>Shipping rates are calculated at checkout based on your location and selected shipping method. Free standard shipping is available on orders over a certain amount.</p>

<h3>International Shipping</h3>
<p>We ship to select international destinations. International shipping times and rates vary by location.</p>

<h3>Tracking Your Order</h3>
<p>Once your order ships, you'll receive an email with tracking information so you can follow your package every step of the way.</p>`,
			MetaTitle:       "Shipping Information",
			MetaDescription: "Learn about our shipping options, delivery times, and tracking.",
			ShowInMenu:      false,
			ShowInFooter:    true,
			IsFeatured:      false,
			ViewCount:       0,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		{
			ID:              "page-faq",
			Type:            "FAQ",
			Status:          "PUBLISHED",
			Title:           "Frequently Asked Questions",
			Slug:            "faq",
			Excerpt:         "Find answers to commonly asked questions.",
			Content:         `<h2>Frequently Asked Questions</h2>

<h3>Orders & Shipping</h3>

<h4>How do I track my order?</h4>
<p>Once your order ships, you'll receive an email with a tracking number. You can use this number to track your package on our website or the carrier's site.</p>

<h4>How long does shipping take?</h4>
<p>Standard shipping typically takes 5-7 business days. Express shipping is available for 2-3 business day delivery.</p>

<h4>Do you ship internationally?</h4>
<p>Yes, we ship to select international destinations. Shipping times and rates vary by location.</p>

<h3>Returns & Refunds</h3>

<h4>What is your return policy?</h4>
<p>We accept returns within 30 days of delivery. Items must be in original condition with tags attached.</p>

<h4>How do I start a return?</h4>
<p>Contact our customer service team and we'll provide you with a return shipping label and instructions.</p>

<h4>When will I receive my refund?</h4>
<p>Refunds are processed within 5-7 business days after we receive your return.</p>

<h3>Account & Orders</h3>

<h4>How do I create an account?</h4>
<p>Click the account icon in the header and select "Create Account." Fill in your details and you're all set!</p>

<h4>I forgot my password. What do I do?</h4>
<p>Click "Forgot Password" on the login page and we'll send you a reset link.</p>

<h3>Still have questions?</h3>
<p>Contact our customer support team and we'll be happy to help!</p>`,
			MetaTitle:       "FAQ - Frequently Asked Questions",
			MetaDescription: "Find answers to commonly asked questions about orders, shipping, returns, and more.",
			ShowInMenu:      false,
			ShowInFooter:    true,
			IsFeatured:      false,
			ViewCount:       0,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		{
			ID:              "page-about-us",
			Type:            "STATIC",
			Status:          "PUBLISHED",
			Title:           "About Us",
			Slug:            "about",
			Excerpt:         "Learn more about our company and mission.",
			Content:         `<h2>About Us</h2>
<p>Welcome to our store! We're passionate about bringing you quality products and exceptional service.</p>

<h3>Our Story</h3>
<p>Founded with a vision to make shopping easier and more enjoyable, we've grown from a small startup to a trusted destination for customers worldwide.</p>

<h3>Our Mission</h3>
<p>We're committed to providing high-quality products at fair prices, backed by outstanding customer service. Every decision we make is guided by our desire to exceed your expectations.</p>

<h3>What Sets Us Apart</h3>
<ul>
<li><strong>Quality Products:</strong> We carefully curate our selection to ensure you receive only the best.</li>
<li><strong>Customer First:</strong> Your satisfaction is our top priority.</li>
<li><strong>Fast Shipping:</strong> We know you're excited about your purchase, so we ship quickly.</li>
<li><strong>Easy Returns:</strong> Not happy? No problem. Our return process is hassle-free.</li>
</ul>

<h3>Join Our Community</h3>
<p>Follow us on social media and sign up for our newsletter to stay updated on new arrivals, special offers, and more!</p>`,
			MetaTitle:       "About Us",
			MetaDescription: "Learn about our company, our mission, and what makes us different.",
			ShowInMenu:      true,
			ShowInFooter:    true,
			IsFeatured:      false,
			ViewCount:       0,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		{
			ID:              "page-contact",
			Type:            "STATIC",
			Status:          "PUBLISHED",
			Title:           "Contact Us",
			Slug:            "contact",
			Excerpt:         "Get in touch with our team.",
			Content:         `<h2>Contact Us</h2>
<p>We'd love to hear from you! Whether you have a question, feedback, or just want to say hello, we're here to help.</p>

<h3>Customer Support</h3>
<p>Our support team is available to assist you with any questions or concerns.</p>
<ul>
<li><strong>Email:</strong> support@example.com</li>
<li><strong>Phone:</strong> 1-800-XXX-XXXX</li>
<li><strong>Hours:</strong> Monday - Friday, 9 AM - 5 PM EST</li>
</ul>

<h3>Response Time</h3>
<p>We aim to respond to all inquiries within 24-48 business hours.</p>

<h3>Business Inquiries</h3>
<p>For partnerships, press, or other business inquiries, please email us at business@example.com.</p>

<h3>Visit Us</h3>
<p>Have a local issue? Feel free to stop by our office during business hours. We're always happy to meet our customers in person!</p>`,
			MetaTitle:       "Contact Us",
			MetaDescription: "Get in touch with our customer support team. We're here to help!",
			ShowInMenu:      true,
			ShowInFooter:    true,
			IsFeatured:      false,
			ViewCount:       0,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}
}

// StorefrontThemeHistory represents a historical version of theme settings
type StorefrontThemeHistory struct {
	ID              uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ThemeSettingsID uuid.UUID      `json:"themeSettingsId" gorm:"type:uuid;not null;index"`
	TenantID        uuid.UUID      `json:"tenantId" gorm:"type:uuid;not null;index"`
	Version         int            `json:"version" gorm:"not null"`
	Snapshot        datatypes.JSON `json:"snapshot" gorm:"type:jsonb;not null"`
	ChangeSummary   *string        `json:"changeSummary,omitempty" gorm:"type:text"`
	CreatedBy       *uuid.UUID     `json:"createdBy,omitempty" gorm:"type:uuid"`
	CreatedAt       time.Time      `json:"createdAt" gorm:"autoCreateTime"`
}

// TableName specifies the table name for StorefrontThemeHistory
func (StorefrontThemeHistory) TableName() string {
	return "storefront_theme_history"
}

// ThemeHistoryListResponse represents the response for listing theme history
type ThemeHistoryListResponse struct {
	Success bool                     `json:"success"`
	Data    []StorefrontThemeHistory `json:"data"`
	Total   int64                    `json:"total"`
	Message string                   `json:"message,omitempty"`
}

// ThemeHistoryResponse represents the response for a single history item
type ThemeHistoryResponse struct {
	Success bool                    `json:"success"`
	Data    *StorefrontThemeHistory `json:"data,omitempty"`
	Message string                  `json:"message,omitempty"`
}

// ========================================
// Enterprise Validation Utilities
// ========================================

// ValidThemeTemplates contains all valid theme template IDs
var ValidThemeTemplates = []string{
	// Core themes
	"vibrant", "minimal", "dark", "neon", "ocean", "sunset",
	"forest", "luxury", "rose", "corporate", "earthy", "arctic",
	// Industry-specific themes
	"fashion", "streetwear", "food", "bakery", "cafe", "electronics",
	"beauty", "wellness", "jewelry", "kids", "sports", "home",
}

// IsValidThemeTemplate validates if a template ID is a known preset
func IsValidThemeTemplate(template string) bool {
	for _, t := range ValidThemeTemplates {
		if t == template {
			return true
		}
	}
	return false
}

// IsValidHexColor validates if a string is a valid hex color code
func IsValidHexColor(color string) bool {
	if len(color) == 0 {
		return false
	}
	// Regex pattern for #RGB or #RRGGBB format
	if len(color) == 4 || len(color) == 7 {
		if color[0] != '#' {
			return false
		}
		for _, c := range color[1:] {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
		return true
	}
	return false
}

// ThemeValidationResult contains the result of theme validation
type ThemeValidationResult struct {
	IsValid  bool     `json:"isValid"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

// ValidateThemeSettings validates the storefront theme settings
func ValidateThemeSettings(settings *CreateStorefrontThemeRequest) *ThemeValidationResult {
	result := &ThemeValidationResult{
		IsValid:  true,
		Errors:   []string{},
		Warnings: []string{},
	}

	// Validate theme template
	if settings.ThemeTemplate != "" && !IsValidThemeTemplate(settings.ThemeTemplate) {
		result.Errors = append(result.Errors, "Invalid theme template: "+settings.ThemeTemplate)
		result.IsValid = false
	}

	// Validate primary color
	if settings.PrimaryColor != "" && !IsValidHexColor(settings.PrimaryColor) {
		result.Errors = append(result.Errors, "Primary color must be a valid hex color (e.g., #8B5CF6)")
		result.IsValid = false
	}

	// Validate secondary color
	if settings.SecondaryColor != "" && !IsValidHexColor(settings.SecondaryColor) {
		result.Errors = append(result.Errors, "Secondary color must be a valid hex color")
		result.IsValid = false
	}

	// Validate accent color if provided
	if settings.AccentColor != nil && *settings.AccentColor != "" && !IsValidHexColor(*settings.AccentColor) {
		result.Errors = append(result.Errors, "Accent color must be a valid hex color")
		result.IsValid = false
	}

	// Validate color mode
	validColorModes := []string{"light", "dark", "both", "system"}
	if settings.ColorMode != "" {
		isValidMode := false
		for _, mode := range validColorModes {
			if mode == settings.ColorMode {
				isValidMode = true
				break
			}
		}
		if !isValidMode {
			result.Errors = append(result.Errors, "Color mode must be one of: light, dark, both, system")
			result.IsValid = false
		}
	}

	return result
}

// GetThemePresetByID retrieves a theme preset by its ID
func GetThemePresetByID(id string) *StorefrontThemePreset {
	for _, preset := range DefaultThemePresets {
		if preset.ID == id {
			return &preset
		}
	}
	return nil
}

// GetDarkThemes returns a list of theme IDs that are dark mode themes
func GetDarkThemes() []string {
	return []string{"dark", "neon", "electronics", "streetwear", "luxury"}
}

// IsDarkTheme checks if a theme template is a dark mode theme
func IsDarkTheme(template string) bool {
	darkThemes := GetDarkThemes()
	for _, t := range darkThemes {
		if t == template {
			return true
		}
	}
	return false
}

// GetIndustryThemes returns all industry-specific theme IDs
func GetIndustryThemes() []string {
	return []string{
		"fashion", "streetwear", "food", "bakery", "cafe",
		"electronics", "beauty", "wellness", "jewelry",
		"kids", "sports", "home",
	}
}

// IsIndustryTheme checks if a theme template is an industry-specific theme
func IsIndustryTheme(template string) bool {
	industryThemes := GetIndustryThemes()
	for _, t := range industryThemes {
		if t == template {
			return true
		}
	}
	return false
}
