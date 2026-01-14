package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"qr-service/internal/config"
	"qr-service/internal/models"

	"github.com/google/uuid"
	qrcode "github.com/skip2/go-qrcode"
)

type StorageInterface interface {
	Upload(ctx context.Context, objectName string, data []byte, contentType string) (string, error)
	Close() error
}

type QRService struct {
	config            *config.QRConfig
	encryptionService *EncryptionService
	storageService    StorageInterface
}

func NewQRService(cfg *config.QRConfig, encryption *EncryptionService, storage StorageInterface) *QRService {
	return &QRService{
		config:            cfg,
		encryptionService: encryption,
		storageService:    storage,
	}
}

func (s *QRService) GenerateQRCode(ctx context.Context, req *models.GenerateQRRequest) (*models.GenerateQRResponse, error) {
	content, err := s.buildQRContent(req.Type, &req.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to build QR content: %w", err)
	}

	size := s.validateSize(req.Size)
	quality := s.validateQuality(req.Quality)
	format := s.validateFormat(req.Format)

	recoveryLevel := s.getRecoveryLevel(quality)

	qr, err := qrcode.New(content, recoveryLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to create QR code: %w", err)
	}

	pngData, err := qr.PNG(size)
	if err != nil {
		return nil, fmt.Errorf("failed to generate PNG: %w", err)
	}

	response := &models.GenerateQRResponse{
		ID:        uuid.New().String(),
		Type:      string(req.Type),
		Format:    string(format),
		Size:      size,
		Quality:   string(quality),
		Encrypted: s.encryptionService != nil && s.isSensitiveType(req.Type),
		CreatedAt: time.Now().UTC(),
	}

	if req.Save && s.storageService != nil {
		objectName := fmt.Sprintf("%s/%s.png", req.TenantID, response.ID)
		storageURL, err := s.storageService.Upload(ctx, objectName, pngData, "image/png")
		if err != nil {
			// Log error but don't fail the request - QR code generation still succeeds
			fmt.Printf("Failed to upload QR code to storage: %v\n", err)
		} else {
			response.StorageURL = storageURL
		}
	}

	if format == models.FormatBase64 {
		response.QRCode = base64.StdEncoding.EncodeToString(pngData)
	}

	return response, nil
}

func (s *QRService) GenerateQRCodePNG(ctx context.Context, qrType models.QRCodeType, data *models.QRData, size int, quality models.QRCodeQuality) ([]byte, error) {
	content, err := s.buildQRContent(qrType, data)
	if err != nil {
		return nil, fmt.Errorf("failed to build QR content: %w", err)
	}

	size = s.validateSize(size)
	quality = s.validateQuality(quality)

	recoveryLevel := s.getRecoveryLevel(quality)

	qr, err := qrcode.New(content, recoveryLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to create QR code: %w", err)
	}

	pngData, err := qr.PNG(size)
	if err != nil {
		return nil, fmt.Errorf("failed to generate PNG: %w", err)
	}

	return pngData, nil
}

func (s *QRService) BatchGenerateQRCodes(ctx context.Context, req *models.BatchGenerateRequest) *models.BatchGenerateResponse {
	results := make([]models.BatchQRResult, len(req.Items))
	successCount := 0
	failedCount := 0

	size := s.validateSize(req.Size)
	quality := s.validateQuality(req.Quality)
	recoveryLevel := s.getRecoveryLevel(quality)

	for i, item := range req.Items {
		content, err := s.buildQRContent(item.Type, &item.Data)
		if err != nil {
			results[i] = models.BatchQRResult{
				Label: item.Label,
				Error: fmt.Sprintf("failed to build content: %v", err),
			}
			failedCount++
			continue
		}

		qr, err := qrcode.New(content, recoveryLevel)
		if err != nil {
			results[i] = models.BatchQRResult{
				Label: item.Label,
				Error: fmt.Sprintf("failed to create QR code: %v", err),
			}
			failedCount++
			continue
		}

		pngData, err := qr.PNG(size)
		if err != nil {
			results[i] = models.BatchQRResult{
				Label: item.Label,
				Error: fmt.Sprintf("failed to generate PNG: %v", err),
			}
			failedCount++
			continue
		}

		result := models.BatchQRResult{
			Label:  item.Label,
			QRCode: base64.StdEncoding.EncodeToString(pngData),
		}

		if req.Save && s.storageService != nil {
			id := uuid.New().String()
			objectName := fmt.Sprintf("batch/%s.png", id)
			storageURL, err := s.storageService.Upload(ctx, objectName, pngData, "image/png")
			if err == nil {
				result.StorageURL = storageURL
			}
		}

		results[i] = result
		successCount++
	}

	return &models.BatchGenerateResponse{
		Results: results,
		Total:   len(req.Items),
		Success: successCount,
		Failed:  failedCount,
	}
}

func (s *QRService) buildQRContent(qrType models.QRCodeType, data *models.QRData) (string, error) {
	switch qrType {
	case models.TypeURL:
		if data.URL == "" {
			return "", fmt.Errorf("URL is required for URL type")
		}
		return data.URL, nil

	case models.TypeText:
		if data.Text == "" {
			return "", fmt.Errorf("text is required for text type")
		}
		return data.Text, nil

	case models.TypeWiFi:
		if data.WiFi == nil || data.WiFi.SSID == "" {
			return "", fmt.Errorf("WiFi SSID is required")
		}
		return s.buildWiFiString(data.WiFi), nil

	case models.TypeVCard:
		if data.VCard == nil {
			return "", fmt.Errorf("VCard data is required")
		}
		return s.buildVCardString(data.VCard), nil

	case models.TypeEmail:
		if data.Email == nil || data.Email.Address == "" {
			return "", fmt.Errorf("email address is required")
		}
		return s.buildEmailString(data.Email), nil

	case models.TypePhone:
		if data.Phone == "" {
			return "", fmt.Errorf("phone number is required")
		}
		return fmt.Sprintf("tel:%s", data.Phone), nil

	case models.TypeSMS:
		if data.SMS == nil || data.SMS.Phone == "" {
			return "", fmt.Errorf("SMS phone number is required")
		}
		return s.buildSMSString(data.SMS), nil

	case models.TypeGeo:
		if data.Geo == nil {
			return "", fmt.Errorf("geo coordinates are required")
		}
		return s.buildGeoString(data.Geo), nil

	case models.TypeApp:
		if data.App == nil {
			return "", fmt.Errorf("app data is required")
		}
		return s.buildAppString(data.App), nil

	case models.TypePayment:
		if data.Payment == nil {
			return "", fmt.Errorf("payment data is required")
		}
		return s.buildPaymentString(data.Payment), nil

	default:
		return "", fmt.Errorf("unsupported QR type: %s", qrType)
	}
}

func (s *QRService) buildWiFiString(wifi *models.WiFiData) string {
	encryption := wifi.Encryption
	if encryption == "" {
		if wifi.Password != "" {
			encryption = "WPA"
		} else {
			encryption = "nopass"
		}
	}

	hidden := ""
	if wifi.Hidden {
		hidden = "H:true;"
	}

	password := ""
	if wifi.Password != "" {
		password = fmt.Sprintf("P:%s;", wifi.Password)
	}

	return fmt.Sprintf("WIFI:T:%s;S:%s;%s%s;", encryption, wifi.SSID, password, hidden)
}

func (s *QRService) buildVCardString(vcard *models.VCardData) string {
	var sb strings.Builder
	sb.WriteString("BEGIN:VCARD\n")
	sb.WriteString("VERSION:3.0\n")

	if vcard.FirstName != "" || vcard.LastName != "" {
		sb.WriteString(fmt.Sprintf("N:%s;%s;;;\n", vcard.LastName, vcard.FirstName))
		sb.WriteString(fmt.Sprintf("FN:%s %s\n", vcard.FirstName, vcard.LastName))
	}

	if vcard.Organization != "" {
		sb.WriteString(fmt.Sprintf("ORG:%s\n", vcard.Organization))
	}

	if vcard.Title != "" {
		sb.WriteString(fmt.Sprintf("TITLE:%s\n", vcard.Title))
	}

	if vcard.Email != "" {
		sb.WriteString(fmt.Sprintf("EMAIL:%s\n", vcard.Email))
	}

	if vcard.Phone != "" {
		sb.WriteString(fmt.Sprintf("TEL;TYPE=WORK:%s\n", vcard.Phone))
	}

	if vcard.Mobile != "" {
		sb.WriteString(fmt.Sprintf("TEL;TYPE=CELL:%s\n", vcard.Mobile))
	}

	if vcard.Address != "" || vcard.City != "" || vcard.State != "" || vcard.Zip != "" || vcard.Country != "" {
		sb.WriteString(fmt.Sprintf("ADR;TYPE=WORK:;;%s;%s;%s;%s;%s\n",
			vcard.Address, vcard.City, vcard.State, vcard.Zip, vcard.Country))
	}

	if vcard.Website != "" {
		sb.WriteString(fmt.Sprintf("URL:%s\n", vcard.Website))
	}

	if vcard.Note != "" {
		sb.WriteString(fmt.Sprintf("NOTE:%s\n", vcard.Note))
	}

	sb.WriteString("END:VCARD")
	return sb.String()
}

func (s *QRService) buildEmailString(email *models.EmailData) string {
	result := fmt.Sprintf("mailto:%s", email.Address)
	params := []string{}

	if email.Subject != "" {
		params = append(params, fmt.Sprintf("subject=%s", email.Subject))
	}
	if email.Body != "" {
		params = append(params, fmt.Sprintf("body=%s", email.Body))
	}

	if len(params) > 0 {
		result += "?" + strings.Join(params, "&")
	}

	return result
}

func (s *QRService) buildSMSString(sms *models.SMSData) string {
	if sms.Message != "" {
		return fmt.Sprintf("sms:%s?body=%s", sms.Phone, sms.Message)
	}
	return fmt.Sprintf("sms:%s", sms.Phone)
}

func (s *QRService) buildGeoString(geo *models.GeoData) string {
	if geo.Altitude != 0 {
		return fmt.Sprintf("geo:%f,%f,%f", geo.Latitude, geo.Longitude, geo.Altitude)
	}
	return fmt.Sprintf("geo:%f,%f", geo.Latitude, geo.Longitude)
}

func (s *QRService) buildAppString(app *models.AppData) string {
	if app.FallbackUrl != "" {
		return app.FallbackUrl
	}
	if app.AndroidUrl != "" {
		return app.AndroidUrl
	}
	if app.IOSUrl != "" {
		return app.IOSUrl
	}
	return ""
}

func (s *QRService) buildPaymentString(payment *models.PaymentData) string {
	switch strings.ToLower(payment.Type) {
	case "upi":
		params := []string{}
		if payment.UPIId != "" {
			params = append(params, fmt.Sprintf("pa=%s", payment.UPIId))
		}
		if payment.Name != "" {
			params = append(params, fmt.Sprintf("pn=%s", payment.Name))
		}
		if payment.Amount > 0 {
			params = append(params, fmt.Sprintf("am=%.2f", payment.Amount))
		}
		if payment.Reference != "" {
			params = append(params, fmt.Sprintf("tr=%s", payment.Reference))
		}
		if payment.Currency != "" {
			params = append(params, fmt.Sprintf("cu=%s", payment.Currency))
		}
		return fmt.Sprintf("upi://pay?%s", strings.Join(params, "&"))

	case "bitcoin":
		result := fmt.Sprintf("bitcoin:%s", payment.Address)
		if payment.Amount > 0 {
			result += fmt.Sprintf("?amount=%f", payment.Amount)
		}
		return result

	case "ethereum":
		result := fmt.Sprintf("ethereum:%s", payment.Address)
		if payment.Amount > 0 {
			result += fmt.Sprintf("?value=%f", payment.Amount)
		}
		return result

	default:
		return payment.Address
	}
}

func (s *QRService) isSensitiveType(qrType models.QRCodeType) bool {
	switch qrType {
	case models.TypeWiFi, models.TypePayment, models.TypeVCard:
		return true
	default:
		return false
	}
}

func (s *QRService) EncryptData(data interface{}) (string, error) {
	if s.encryptionService == nil {
		return "", fmt.Errorf("encryption service not available")
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal data: %w", err)
	}

	return s.encryptionService.Encrypt(string(jsonData))
}

func (s *QRService) HashData(data interface{}) (string, error) {
	if s.encryptionService == nil {
		return "", fmt.Errorf("encryption service not available")
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal data: %w", err)
	}

	return s.encryptionService.Hash(string(jsonData)), nil
}

func (s *QRService) validateSize(size int) int {
	if size <= 0 {
		return s.config.DefaultSize
	}
	if size < s.config.MinSize {
		return s.config.MinSize
	}
	if size > s.config.MaxSize {
		return s.config.MaxSize
	}
	return size
}

func (s *QRService) validateQuality(quality models.QRCodeQuality) models.QRCodeQuality {
	switch quality {
	case models.QualityLow, models.QualityMedium, models.QualityHigh, models.QualityHighest:
		return quality
	default:
		return models.QRCodeQuality(s.config.DefaultQuality)
	}
}

func (s *QRService) validateFormat(format models.QRCodeFormat) models.QRCodeFormat {
	switch format {
	case models.FormatPNG, models.FormatBase64:
		return format
	default:
		return models.FormatBase64
	}
}

func (s *QRService) getRecoveryLevel(quality models.QRCodeQuality) qrcode.RecoveryLevel {
	switch quality {
	case models.QualityLow:
		return qrcode.Low
	case models.QualityMedium:
		return qrcode.Medium
	case models.QualityHigh:
		return qrcode.High
	case models.QualityHighest:
		return qrcode.Highest
	default:
		return qrcode.Medium
	}
}
