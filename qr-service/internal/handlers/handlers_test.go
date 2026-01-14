package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"qr-service/internal/config"
	"qr-service/internal/models"
	"qr-service/internal/services"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestRouter() (*gin.Engine, *QRHandlers, *HealthHandlers) {
	cfg := &config.QRConfig{
		DefaultSize:    256,
		MaxSize:        1024,
		MinSize:        64,
		DefaultQuality: "medium",
	}

	qrService := services.NewQRService(cfg, nil, nil)
	qrHandlers := NewQRHandlers(qrService, nil)
	healthHandlers := NewHealthHandlers()

	router := gin.New()
	router.Use(gin.Recovery())

	// Add request ID middleware for testing
	router.Use(func(c *gin.Context) {
		c.Set("request_id", "test-request-id")
		c.Next()
	})

	router.GET("/health", healthHandlers.Health)
	router.GET("/ready", healthHandlers.Ready)

	v1 := router.Group("/api/v1")
	{
		qr := v1.Group("/qr")
		{
			qr.POST("/generate", qrHandlers.GenerateQRCode)
			qr.GET("/image", qrHandlers.GenerateQRCodeImage)
			qr.POST("/download", qrHandlers.DownloadQRCode)
			qr.POST("/batch", qrHandlers.BatchGenerateQRCodes)
			qr.GET("/types", qrHandlers.GetQRTypes)
		}
	}

	return router, qrHandlers, healthHandlers
}

func TestHealthEndpoint(t *testing.T) {
	router, _, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp models.HealthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Status != "healthy" {
		t.Errorf("Expected status 'healthy', got '%s'", resp.Status)
	}

	if resp.Service != "qr-service" {
		t.Errorf("Expected service 'qr-service', got '%s'", resp.Service)
	}
}

func TestReadyEndpoint(t *testing.T) {
	router, _, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ready", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp models.HealthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Status != "ready" {
		t.Errorf("Expected status 'ready', got '%s'", resp.Status)
	}
}

func TestGenerateQRCode_Success(t *testing.T) {
	router, _, _ := setupTestRouter()

	reqBody := models.GenerateQRRequest{
		Type: models.TypeURL,
		Data: models.QRData{
			URL: "https://example.com",
		},
		Size:    256,
		Quality: models.QualityMedium,
	}

	body, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/qr/generate", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp models.GenerateQRResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
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
}

func TestGenerateQRCode_InvalidJSON(t *testing.T) {
	router, _, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/qr/generate", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestGenerateQRCode_WiFi(t *testing.T) {
	router, _, _ := setupTestRouter()

	reqBody := models.GenerateQRRequest{
		Type: models.TypeWiFi,
		Data: models.QRData{
			WiFi: &models.WiFiData{
				SSID:       "TestNetwork",
				Password:   "secret123",
				Encryption: "WPA",
			},
		},
	}

	body, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/qr/generate", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGenerateQRCodeImage_URL(t *testing.T) {
	router, _, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/qr/image?type=url&url=https://example.com&size=256", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "image/png" {
		t.Errorf("Expected Content-Type 'image/png', got '%s'", contentType)
	}

	// Check PNG magic bytes
	body := w.Body.Bytes()
	if len(body) < 8 {
		t.Error("Response too short to be a PNG")
	}
	pngMagic := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	for i, b := range pngMagic {
		if body[i] != b {
			t.Errorf("Invalid PNG magic byte at position %d", i)
		}
	}
}

func TestGenerateQRCodeImage_MissingURL(t *testing.T) {
	router, _, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/qr/image?type=url", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestGenerateQRCodeImage_Text(t *testing.T) {
	router, _, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/qr/image?type=text&text=Hello", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGenerateQRCodeImage_Phone(t *testing.T) {
	router, _, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/qr/image?type=phone&phone=+1234567890", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGenerateQRCodeImage_ComplexType(t *testing.T) {
	router, _, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/qr/image?type=wifi", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for complex type, got %d", w.Code)
	}
}

func TestDownloadQRCode(t *testing.T) {
	router, _, _ := setupTestRouter()

	reqBody := models.GenerateQRRequest{
		Type: models.TypeURL,
		Data: models.QRData{
			URL: "https://example.com",
		},
	}

	body, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/qr/download?filename=test.png", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	disposition := w.Header().Get("Content-Disposition")
	if disposition != `attachment; filename="test.png"` {
		t.Errorf("Expected Content-Disposition with filename, got '%s'", disposition)
	}
}

func TestBatchGenerateQRCodes(t *testing.T) {
	router, _, _ := setupTestRouter()

	reqBody := models.BatchGenerateRequest{
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
		},
		Size:    256,
		Quality: models.QualityMedium,
	}

	body, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/qr/batch", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp models.BatchGenerateResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Total != 2 {
		t.Errorf("Expected total 2, got %d", resp.Total)
	}

	if resp.Success != 2 {
		t.Errorf("Expected success 2, got %d", resp.Success)
	}
}

func TestGetQRTypes(t *testing.T) {
	router, _, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/qr/types", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	types, ok := resp["types"].([]interface{})
	if !ok {
		t.Fatal("Expected 'types' array in response")
	}

	if len(types) != 10 {
		t.Errorf("Expected 10 types, got %d", len(types))
	}

	// Verify expected types are present
	expectedTypes := []string{"url", "text", "wifi", "vcard", "email", "phone", "sms", "geo", "app", "payment"}
	for _, expected := range expectedTypes {
		found := false
		for _, t := range types {
			typeMap := t.(map[string]interface{})
			if typeMap["type"] == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected type '%s' not found", expected)
		}
	}
}

func TestGenerateQRCode_WithTenantID(t *testing.T) {
	router, _, _ := setupTestRouter()

	reqBody := models.GenerateQRRequest{
		Type: models.TypeURL,
		Data: models.QRData{
			URL: "https://example.com",
		},
	}

	body, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/qr/generate", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-123")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGenerateQRCode_AllTypes(t *testing.T) {
	router, _, _ := setupTestRouter()

	testCases := []struct {
		name string
		req  models.GenerateQRRequest
	}{
		{
			name: "URL",
			req: models.GenerateQRRequest{
				Type: models.TypeURL,
				Data: models.QRData{URL: "https://example.com"},
			},
		},
		{
			name: "Text",
			req: models.GenerateQRRequest{
				Type: models.TypeText,
				Data: models.QRData{Text: "Hello World"},
			},
		},
		{
			name: "Phone",
			req: models.GenerateQRRequest{
				Type: models.TypePhone,
				Data: models.QRData{Phone: "+1234567890"},
			},
		},
		{
			name: "Email",
			req: models.GenerateQRRequest{
				Type: models.TypeEmail,
				Data: models.QRData{Email: &models.EmailData{Address: "test@example.com"}},
			},
		},
		{
			name: "SMS",
			req: models.GenerateQRRequest{
				Type: models.TypeSMS,
				Data: models.QRData{SMS: &models.SMSData{Phone: "+1234567890"}},
			},
		},
		{
			name: "Geo",
			req: models.GenerateQRRequest{
				Type: models.TypeGeo,
				Data: models.QRData{Geo: &models.GeoData{Latitude: 40.7128, Longitude: -74.0060}},
			},
		},
		{
			name: "WiFi",
			req: models.GenerateQRRequest{
				Type: models.TypeWiFi,
				Data: models.QRData{WiFi: &models.WiFiData{SSID: "TestNetwork"}},
			},
		},
		{
			name: "VCard",
			req: models.GenerateQRRequest{
				Type: models.TypeVCard,
				Data: models.QRData{VCard: &models.VCardData{FirstName: "John", LastName: "Doe"}},
			},
		},
		{
			name: "App",
			req: models.GenerateQRRequest{
				Type: models.TypeApp,
				Data: models.QRData{App: &models.AppData{FallbackUrl: "https://example.com"}},
			},
		},
		{
			name: "Payment UPI",
			req: models.GenerateQRRequest{
				Type: models.TypePayment,
				Data: models.QRData{Payment: &models.PaymentData{Type: "upi", UPIId: "test@upi"}},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.req)
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/v1/qr/generate", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200 for %s, got %d: %s", tc.name, w.Code, w.Body.String())
			}
		})
	}
}
