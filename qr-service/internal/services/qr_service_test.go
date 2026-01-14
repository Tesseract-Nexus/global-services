package services

import (
	"context"
	"strings"
	"testing"

	"qr-service/internal/config"
	"qr-service/internal/models"
)

func newTestQRService() *QRService {
	cfg := &config.QRConfig{
		DefaultSize:    256,
		MaxSize:        1024,
		MinSize:        64,
		DefaultQuality: "medium",
	}
	return NewQRService(cfg, nil, nil)
}

func TestGenerateQRCode_URL(t *testing.T) {
	svc := newTestQRService()
	ctx := context.Background()

	req := &models.GenerateQRRequest{
		Type: models.TypeURL,
		Data: models.QRData{
			URL: "https://example.com",
		},
		Size:    256,
		Quality: models.QualityMedium,
		Format:  models.FormatBase64,
	}

	resp, err := svc.GenerateQRCode(ctx, req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp.ID == "" {
		t.Error("Expected ID to be set")
	}

	if resp.Type != "url" {
		t.Errorf("Expected type 'url', got '%s'", resp.Type)
	}

	if resp.QRCode == "" {
		t.Error("Expected QRCode to be set")
	}

	if resp.Size != 256 {
		t.Errorf("Expected size 256, got %d", resp.Size)
	}
}

func TestGenerateQRCode_Text(t *testing.T) {
	svc := newTestQRService()
	ctx := context.Background()

	req := &models.GenerateQRRequest{
		Type: models.TypeText,
		Data: models.QRData{
			Text: "Hello, World!",
		},
	}

	resp, err := svc.GenerateQRCode(ctx, req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp.Type != "text" {
		t.Errorf("Expected type 'text', got '%s'", resp.Type)
	}
}

func TestGenerateQRCode_WiFi(t *testing.T) {
	svc := newTestQRService()
	ctx := context.Background()

	req := &models.GenerateQRRequest{
		Type: models.TypeWiFi,
		Data: models.QRData{
			WiFi: &models.WiFiData{
				SSID:       "TestNetwork",
				Password:   "secret123",
				Encryption: "WPA",
				Hidden:     false,
			},
		},
	}

	resp, err := svc.GenerateQRCode(ctx, req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp.Type != "wifi" {
		t.Errorf("Expected type 'wifi', got '%s'", resp.Type)
	}
}

func TestGenerateQRCode_VCard(t *testing.T) {
	svc := newTestQRService()
	ctx := context.Background()

	req := &models.GenerateQRRequest{
		Type: models.TypeVCard,
		Data: models.QRData{
			VCard: &models.VCardData{
				FirstName:    "John",
				LastName:     "Doe",
				Email:        "john.doe@example.com",
				Phone:        "+1234567890",
				Organization: "Acme Inc",
			},
		},
	}

	resp, err := svc.GenerateQRCode(ctx, req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp.Type != "vcard" {
		t.Errorf("Expected type 'vcard', got '%s'", resp.Type)
	}
}

func TestGenerateQRCode_Email(t *testing.T) {
	svc := newTestQRService()
	ctx := context.Background()

	req := &models.GenerateQRRequest{
		Type: models.TypeEmail,
		Data: models.QRData{
			Email: &models.EmailData{
				Address: "test@example.com",
				Subject: "Hello",
				Body:    "Test message",
			},
		},
	}

	resp, err := svc.GenerateQRCode(ctx, req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp.Type != "email" {
		t.Errorf("Expected type 'email', got '%s'", resp.Type)
	}
}

func TestGenerateQRCode_Phone(t *testing.T) {
	svc := newTestQRService()
	ctx := context.Background()

	req := &models.GenerateQRRequest{
		Type: models.TypePhone,
		Data: models.QRData{
			Phone: "+1234567890",
		},
	}

	resp, err := svc.GenerateQRCode(ctx, req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp.Type != "phone" {
		t.Errorf("Expected type 'phone', got '%s'", resp.Type)
	}
}

func TestGenerateQRCode_SMS(t *testing.T) {
	svc := newTestQRService()
	ctx := context.Background()

	req := &models.GenerateQRRequest{
		Type: models.TypeSMS,
		Data: models.QRData{
			SMS: &models.SMSData{
				Phone:   "+1234567890",
				Message: "Hello from QR!",
			},
		},
	}

	resp, err := svc.GenerateQRCode(ctx, req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp.Type != "sms" {
		t.Errorf("Expected type 'sms', got '%s'", resp.Type)
	}
}

func TestGenerateQRCode_Geo(t *testing.T) {
	svc := newTestQRService()
	ctx := context.Background()

	req := &models.GenerateQRRequest{
		Type: models.TypeGeo,
		Data: models.QRData{
			Geo: &models.GeoData{
				Latitude:  40.7128,
				Longitude: -74.0060,
			},
		},
	}

	resp, err := svc.GenerateQRCode(ctx, req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp.Type != "geo" {
		t.Errorf("Expected type 'geo', got '%s'", resp.Type)
	}
}

func TestGenerateQRCode_Payment_UPI(t *testing.T) {
	svc := newTestQRService()
	ctx := context.Background()

	req := &models.GenerateQRRequest{
		Type: models.TypePayment,
		Data: models.QRData{
			Payment: &models.PaymentData{
				Type:     "upi",
				UPIId:    "merchant@upi",
				Name:     "Test Merchant",
				Amount:   100.50,
				Currency: "INR",
			},
		},
	}

	resp, err := svc.GenerateQRCode(ctx, req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp.Type != "payment" {
		t.Errorf("Expected type 'payment', got '%s'", resp.Type)
	}
}

func TestGenerateQRCode_Payment_Bitcoin(t *testing.T) {
	svc := newTestQRService()
	ctx := context.Background()

	req := &models.GenerateQRRequest{
		Type: models.TypePayment,
		Data: models.QRData{
			Payment: &models.PaymentData{
				Type:    "bitcoin",
				Address: "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2",
				Amount:  0.001,
			},
		},
	}

	resp, err := svc.GenerateQRCode(ctx, req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp.Type != "payment" {
		t.Errorf("Expected type 'payment', got '%s'", resp.Type)
	}
}

func TestGenerateQRCodePNG(t *testing.T) {
	svc := newTestQRService()
	ctx := context.Background()

	data := &models.QRData{
		URL: "https://example.com",
	}

	pngData, err := svc.GenerateQRCodePNG(ctx, models.TypeURL, data, 256, models.QualityMedium)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(pngData) == 0 {
		t.Error("Expected PNG data to be non-empty")
	}

	// Check PNG magic bytes
	if len(pngData) < 8 {
		t.Error("PNG data too short")
	}
	pngMagic := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	for i, b := range pngMagic {
		if pngData[i] != b {
			t.Errorf("Invalid PNG magic byte at position %d", i)
		}
	}
}

func TestBatchGenerateQRCodes(t *testing.T) {
	svc := newTestQRService()
	ctx := context.Background()

	req := &models.BatchGenerateRequest{
		Items: []models.BatchQRItem{
			{
				Type:  models.TypeURL,
				Data:  models.QRData{URL: "https://example1.com"},
				Label: "Example 1",
			},
			{
				Type:  models.TypeURL,
				Data:  models.QRData{URL: "https://example2.com"},
				Label: "Example 2",
			},
			{
				Type:  models.TypeText,
				Data:  models.QRData{Text: "Hello"},
				Label: "Text QR",
			},
		},
		Size:    256,
		Quality: models.QualityMedium,
	}

	resp := svc.BatchGenerateQRCodes(ctx, req)

	if resp.Total != 3 {
		t.Errorf("Expected total 3, got %d", resp.Total)
	}

	if resp.Success != 3 {
		t.Errorf("Expected success 3, got %d", resp.Success)
	}

	if resp.Failed != 0 {
		t.Errorf("Expected failed 0, got %d", resp.Failed)
	}

	if len(resp.Results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(resp.Results))
	}

	for i, result := range resp.Results {
		if result.QRCode == "" {
			t.Errorf("Expected QRCode for result %d", i)
		}
		if result.Error != "" {
			t.Errorf("Unexpected error for result %d: %s", i, result.Error)
		}
	}
}

func TestBatchGenerateQRCodes_WithErrors(t *testing.T) {
	svc := newTestQRService()
	ctx := context.Background()

	req := &models.BatchGenerateRequest{
		Items: []models.BatchQRItem{
			{
				Type:  models.TypeURL,
				Data:  models.QRData{URL: "https://example.com"},
				Label: "Valid",
			},
			{
				Type:  models.TypeURL,
				Data:  models.QRData{URL: ""}, // Missing URL
				Label: "Invalid",
			},
		},
	}

	resp := svc.BatchGenerateQRCodes(ctx, req)

	if resp.Total != 2 {
		t.Errorf("Expected total 2, got %d", resp.Total)
	}

	if resp.Success != 1 {
		t.Errorf("Expected success 1, got %d", resp.Success)
	}

	if resp.Failed != 1 {
		t.Errorf("Expected failed 1, got %d", resp.Failed)
	}
}

func TestValidateSize(t *testing.T) {
	svc := newTestQRService()

	tests := []struct {
		input    int
		expected int
	}{
		{0, 256},     // Default
		{-1, 256},    // Negative becomes default
		{50, 64},     // Below min becomes min
		{64, 64},     // At min stays
		{256, 256},   // Normal size
		{1024, 1024}, // At max stays
		{2000, 1024}, // Above max becomes max
	}

	for _, tt := range tests {
		result := svc.validateSize(tt.input)
		if result != tt.expected {
			t.Errorf("validateSize(%d) = %d, expected %d", tt.input, result, tt.expected)
		}
	}
}

func TestValidateQuality(t *testing.T) {
	svc := newTestQRService()

	tests := []struct {
		input    models.QRCodeQuality
		expected models.QRCodeQuality
	}{
		{models.QualityLow, models.QualityLow},
		{models.QualityMedium, models.QualityMedium},
		{models.QualityHigh, models.QualityHigh},
		{models.QualityHighest, models.QualityHighest},
		{"invalid", models.QualityMedium}, // Invalid becomes default
		{"", models.QualityMedium},        // Empty becomes default
	}

	for _, tt := range tests {
		result := svc.validateQuality(tt.input)
		if result != tt.expected {
			t.Errorf("validateQuality(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}

func TestBuildWiFiString(t *testing.T) {
	svc := newTestQRService()

	tests := []struct {
		name     string
		wifi     *models.WiFiData
		contains []string
	}{
		{
			name: "WPA with password",
			wifi: &models.WiFiData{
				SSID:       "TestNetwork",
				Password:   "secret123",
				Encryption: "WPA",
			},
			contains: []string{"WIFI:", "T:WPA", "S:TestNetwork", "P:secret123"},
		},
		{
			name: "Open network",
			wifi: &models.WiFiData{
				SSID:       "OpenNetwork",
				Encryption: "nopass",
			},
			contains: []string{"WIFI:", "T:nopass", "S:OpenNetwork"},
		},
		{
			name: "Hidden network",
			wifi: &models.WiFiData{
				SSID:       "HiddenNetwork",
				Password:   "secret",
				Encryption: "WPA",
				Hidden:     true,
			},
			contains: []string{"WIFI:", "H:true"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.buildWiFiString(tt.wifi)
			for _, substr := range tt.contains {
				if !strings.Contains(result, substr) {
					t.Errorf("Expected '%s' to contain '%s'", result, substr)
				}
			}
		})
	}
}

func TestBuildVCardString(t *testing.T) {
	svc := newTestQRService()

	vcard := &models.VCardData{
		FirstName:    "John",
		LastName:     "Doe",
		Email:        "john@example.com",
		Phone:        "+1234567890",
		Organization: "Acme Inc",
		Title:        "Developer",
	}

	result := svc.buildVCardString(vcard)

	expectedContains := []string{
		"BEGIN:VCARD",
		"VERSION:3.0",
		"N:Doe;John",
		"FN:John Doe",
		"EMAIL:john@example.com",
		"TEL;TYPE=WORK:+1234567890",
		"ORG:Acme Inc",
		"TITLE:Developer",
		"END:VCARD",
	}

	for _, substr := range expectedContains {
		if !strings.Contains(result, substr) {
			t.Errorf("Expected vCard to contain '%s'", substr)
		}
	}
}

func TestBuildPaymentString_UPI(t *testing.T) {
	svc := newTestQRService()

	payment := &models.PaymentData{
		Type:     "upi",
		UPIId:    "merchant@upi",
		Name:     "Test Store",
		Amount:   100.50,
		Currency: "INR",
	}

	result := svc.buildPaymentString(payment)

	expectedContains := []string{
		"upi://pay?",
		"pa=merchant@upi",
		"pn=Test Store",
		"am=100.50",
		"cu=INR",
	}

	for _, substr := range expectedContains {
		if !strings.Contains(result, substr) {
			t.Errorf("Expected UPI string to contain '%s', got '%s'", substr, result)
		}
	}
}

func TestGenerateQRCode_MissingData(t *testing.T) {
	svc := newTestQRService()
	ctx := context.Background()

	tests := []struct {
		name string
		req  *models.GenerateQRRequest
	}{
		{
			name: "Missing URL",
			req: &models.GenerateQRRequest{
				Type: models.TypeURL,
				Data: models.QRData{},
			},
		},
		{
			name: "Missing Text",
			req: &models.GenerateQRRequest{
				Type: models.TypeText,
				Data: models.QRData{},
			},
		},
		{
			name: "Missing WiFi data",
			req: &models.GenerateQRRequest{
				Type: models.TypeWiFi,
				Data: models.QRData{},
			},
		},
		{
			name: "Missing VCard data",
			req: &models.GenerateQRRequest{
				Type: models.TypeVCard,
				Data: models.QRData{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.GenerateQRCode(ctx, tt.req)
			if err == nil {
				t.Error("Expected error for missing data")
			}
		})
	}
}

func TestIsSensitiveType(t *testing.T) {
	svc := newTestQRService()

	sensitiveTypes := []models.QRCodeType{
		models.TypeWiFi,
		models.TypePayment,
		models.TypeVCard,
	}

	nonSensitiveTypes := []models.QRCodeType{
		models.TypeURL,
		models.TypeText,
		models.TypePhone,
		models.TypeEmail,
		models.TypeSMS,
		models.TypeGeo,
		models.TypeApp,
	}

	for _, qrType := range sensitiveTypes {
		if !svc.isSensitiveType(qrType) {
			t.Errorf("Expected %s to be sensitive", qrType)
		}
	}

	for _, qrType := range nonSensitiveTypes {
		if svc.isSensitiveType(qrType) {
			t.Errorf("Expected %s to be non-sensitive", qrType)
		}
	}
}
