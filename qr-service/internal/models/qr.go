package models

import (
	"time"

	"github.com/google/uuid"
)

type QRCodeFormat string
type QRCodeQuality string
type QRCodeType string

const (
	FormatPNG    QRCodeFormat = "png"
	FormatBase64 QRCodeFormat = "base64"

	QualityLow     QRCodeQuality = "low"
	QualityMedium  QRCodeQuality = "medium"
	QualityHigh    QRCodeQuality = "high"
	QualityHighest QRCodeQuality = "highest"

	TypeURL     QRCodeType = "url"
	TypeWiFi    QRCodeType = "wifi"
	TypeVCard   QRCodeType = "vcard"
	TypeText    QRCodeType = "text"
	TypeEmail   QRCodeType = "email"
	TypePhone   QRCodeType = "phone"
	TypeSMS     QRCodeType = "sms"
	TypeGeo     QRCodeType = "geo"
	TypeApp     QRCodeType = "app"
	TypePayment QRCodeType = "payment"
)

type GenerateQRRequest struct {
	Type     QRCodeType    `json:"type" binding:"required"`
	Data     QRData        `json:"data" binding:"required"`
	Size     int           `json:"size,omitempty"`
	Format   QRCodeFormat  `json:"format,omitempty"`
	Quality  QRCodeQuality `json:"quality,omitempty"`
	TenantID string        `json:"tenant_id,omitempty"`
	Label    string        `json:"label,omitempty"`
	Save     bool          `json:"save,omitempty"`
}

type QRData struct {
	URL      string    `json:"url,omitempty"`
	Text     string    `json:"text,omitempty"`
	WiFi     *WiFiData `json:"wifi,omitempty"`
	VCard    *VCardData `json:"vcard,omitempty"`
	Email    *EmailData `json:"email,omitempty"`
	Phone    string    `json:"phone,omitempty"`
	SMS      *SMSData  `json:"sms,omitempty"`
	Geo      *GeoData  `json:"geo,omitempty"`
	App      *AppData  `json:"app,omitempty"`
	Payment  *PaymentData `json:"payment,omitempty"`
}

type WiFiData struct {
	SSID       string `json:"ssid" binding:"required"`
	Password   string `json:"password,omitempty"`
	Encryption string `json:"encryption,omitempty"`
	Hidden     bool   `json:"hidden,omitempty"`
}

type VCardData struct {
	FirstName    string `json:"first_name,omitempty"`
	LastName     string `json:"last_name,omitempty"`
	Organization string `json:"organization,omitempty"`
	Title        string `json:"title,omitempty"`
	Email        string `json:"email,omitempty"`
	Phone        string `json:"phone,omitempty"`
	Mobile       string `json:"mobile,omitempty"`
	Address      string `json:"address,omitempty"`
	City         string `json:"city,omitempty"`
	State        string `json:"state,omitempty"`
	Zip          string `json:"zip,omitempty"`
	Country      string `json:"country,omitempty"`
	Website      string `json:"website,omitempty"`
	Note         string `json:"note,omitempty"`
}

type EmailData struct {
	Address string `json:"address" binding:"required,email"`
	Subject string `json:"subject,omitempty"`
	Body    string `json:"body,omitempty"`
}

type SMSData struct {
	Phone   string `json:"phone" binding:"required"`
	Message string `json:"message,omitempty"`
}

type GeoData struct {
	Latitude  float64 `json:"latitude" binding:"required"`
	Longitude float64 `json:"longitude" binding:"required"`
	Altitude  float64 `json:"altitude,omitempty"`
}

type AppData struct {
	IOSUrl     string `json:"ios_url,omitempty"`
	AndroidUrl string `json:"android_url,omitempty"`
	FallbackUrl string `json:"fallback_url,omitempty"`
}

type PaymentData struct {
	Type      string  `json:"type" binding:"required"`
	Address   string  `json:"address,omitempty"`
	Amount    float64 `json:"amount,omitempty"`
	Currency  string  `json:"currency,omitempty"`
	Reference string  `json:"reference,omitempty"`
	UPIId     string  `json:"upi_id,omitempty"`
	Name      string  `json:"name,omitempty"`
}

type GenerateQRResponse struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	QRCode    string    `json:"qr_code,omitempty"`
	StorageURL string   `json:"storage_url,omitempty"`
	Format    string    `json:"format"`
	Size      int       `json:"size"`
	Quality   string    `json:"quality"`
	Encrypted bool      `json:"encrypted"`
	CreatedAt time.Time `json:"created_at"`
}

type QRCodeRecord struct {
	ID           uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID     string     `json:"tenant_id" gorm:"type:varchar(255);not null;index"`
	Type         string     `json:"type" gorm:"type:varchar(50);not null"`
	Label        string     `json:"label,omitempty" gorm:"type:varchar(255)"`
	DataHash     string     `json:"data_hash" gorm:"type:varchar(64);not null;index"`
	EncryptedData string    `json:"encrypted_data,omitempty" gorm:"type:text"`
	StorageURL   string     `json:"storage_url,omitempty" gorm:"type:text"`
	Size         int        `json:"size" gorm:"type:int;not null"`
	Quality      string     `json:"quality" gorm:"type:varchar(20);not null"`
	Format       string     `json:"format" gorm:"type:varchar(20);not null"`
	CreatedAt    time.Time  `json:"created_at" gorm:"default:CURRENT_TIMESTAMP"`
	CreatedBy    string     `json:"created_by,omitempty" gorm:"type:varchar(255)"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty" gorm:"type:timestamp"`
	AccessCount  int        `json:"access_count" gorm:"type:int;default:0"`
}

func (QRCodeRecord) TableName() string {
	return "qr_codes"
}

type BatchGenerateRequest struct {
	Items   []BatchQRItem `json:"items" binding:"required,min=1,max=50,dive"`
	Size    int           `json:"size,omitempty"`
	Quality QRCodeQuality `json:"quality,omitempty"`
	Format  QRCodeFormat  `json:"format,omitempty"`
	Save    bool          `json:"save,omitempty"`
}

type BatchQRItem struct {
	Type  QRCodeType `json:"type" binding:"required"`
	Data  QRData     `json:"data" binding:"required"`
	Label string     `json:"label,omitempty"`
}

type BatchGenerateResponse struct {
	Results []BatchQRResult `json:"results"`
	Total   int             `json:"total"`
	Success int             `json:"success"`
	Failed  int             `json:"failed"`
}

type BatchQRResult struct {
	Label      string `json:"label,omitempty"`
	QRCode     string `json:"qr_code,omitempty"`
	StorageURL string `json:"storage_url,omitempty"`
	Error      string `json:"error,omitempty"`
}

type ListQRCodesRequest struct {
	TenantID string `form:"tenant_id"`
	Type     string `form:"type"`
	Page     int    `form:"page,default=1"`
	Limit    int    `form:"limit,default=20"`
}

type ListQRCodesResponse struct {
	Items      []QRCodeRecord `json:"items"`
	Total      int64          `json:"total"`
	Page       int            `json:"page"`
	Limit      int            `json:"limit"`
	TotalPages int            `json:"total_pages"`
}

// UploadQRRequest is the request to upload a composite QR image (with logo)
type UploadQRRequest struct {
	QRID        string `json:"qr_id" binding:"required"`
	ImageBase64 string `json:"image_base64" binding:"required"`
	LogoBase64  string `json:"logo_base64,omitempty"`
	ContentType string `json:"content_type,omitempty"`
}

// UploadQRResponse is the response after uploading a composite QR image
type UploadQRResponse struct {
	ID         string `json:"id"`
	StorageURL string `json:"storage_url"`
	LogoURL    string `json:"logo_url,omitempty"`
}

type HealthResponse struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Version   string `json:"version"`
	Timestamp string `json:"timestamp"`
}

type ErrorResponse struct {
	Error     string `json:"error"`
	Code      string `json:"code,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}
