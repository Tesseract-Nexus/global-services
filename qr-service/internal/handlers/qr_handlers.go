package handlers

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"

	"qr-service/internal/models"
	"qr-service/internal/services"

	"github.com/gin-gonic/gin"
)

// QRHandlers handles QR code generation requests
type QRHandlers struct {
	qrService      *services.QRService
	storageService services.StorageInterface
}

// NewQRHandlers creates a new QRHandlers instance
func NewQRHandlers(qrService *services.QRService, storageService services.StorageInterface) *QRHandlers {
	return &QRHandlers{
		qrService:      qrService,
		storageService: storageService,
	}
}

// GenerateQRCode godoc
// @Summary Generate a QR code
// @Description Generate a QR code for the specified type and data. Supports URL, WiFi, vCard, email, phone, SMS, geo, app, and payment types.
// @Tags QR Code
// @Accept json
// @Produce json
// @Param X-Tenant-ID header string false "Tenant ID for multi-tenant setups"
// @Param X-Request-ID header string false "Request ID for tracing"
// @Param request body models.GenerateQRRequest true "QR Code generation request"
// @Success 200 {object} models.GenerateQRResponse "QR code generated successfully"
// @Failure 400 {object} models.ErrorResponse "Invalid request"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/qr/generate [post]
func (h *QRHandlers) GenerateQRCode(c *gin.Context) {
	var req models.GenerateQRRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:     fmt.Sprintf("Invalid request: %v", err),
			Code:      "INVALID_REQUEST",
			RequestID: c.GetString("request_id"),
		})
		return
	}

	if req.TenantID == "" {
		req.TenantID = c.GetHeader("X-Tenant-ID")
	}

	response, err := h.qrService.GenerateQRCode(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:     fmt.Sprintf("Failed to generate QR code: %v", err),
			Code:      "GENERATION_FAILED",
			RequestID: c.GetString("request_id"),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GenerateQRCodeImage godoc
// @Summary Generate QR code as PNG image
// @Description Generate a QR code and return it directly as a PNG image. Only supports simple types (url, text, phone).
// @Tags QR Code
// @Produce image/png
// @Param type query string false "QR code type" Enums(url, text, phone) default(url)
// @Param url query string false "URL for URL type QR codes"
// @Param text query string false "Text for text type QR codes"
// @Param phone query string false "Phone number for phone type QR codes"
// @Param size query int false "Size of QR code in pixels" minimum(64) maximum(1024) default(256)
// @Param quality query string false "Error correction level" Enums(low, medium, high, highest) default(medium)
// @Success 200 {file} binary "QR code PNG image"
// @Failure 400 {object} models.ErrorResponse "Invalid request"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/qr/image [get]
func (h *QRHandlers) GenerateQRCodeImage(c *gin.Context) {
	qrType := models.QRCodeType(c.DefaultQuery("type", "url"))

	data := &models.QRData{}

	switch qrType {
	case models.TypeURL:
		url := c.Query("url")
		if url == "" {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:     "URL parameter is required for URL type",
				Code:      "MISSING_URL",
				RequestID: c.GetString("request_id"),
			})
			return
		}
		data.URL = url
	case models.TypeText:
		text := c.Query("text")
		if text == "" {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:     "Text parameter is required for text type",
				Code:      "MISSING_TEXT",
				RequestID: c.GetString("request_id"),
			})
			return
		}
		data.Text = text
	case models.TypePhone:
		phone := c.Query("phone")
		if phone == "" {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:     "Phone parameter is required for phone type",
				Code:      "MISSING_PHONE",
				RequestID: c.GetString("request_id"),
			})
			return
		}
		data.Phone = phone
	default:
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:     "Use POST /api/v1/qr/generate for complex QR types",
			Code:      "USE_POST",
			RequestID: c.GetString("request_id"),
		})
		return
	}

	size := 256
	if sizeParam := c.Query("size"); sizeParam != "" {
		if parsedSize, err := strconv.Atoi(sizeParam); err == nil {
			size = parsedSize
		}
	}

	quality := models.QRCodeQuality(c.DefaultQuery("quality", "medium"))

	pngData, err := h.qrService.GenerateQRCodePNG(c.Request.Context(), qrType, data, size, quality)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:     fmt.Sprintf("Failed to generate QR code: %v", err),
			Code:      "GENERATION_FAILED",
			RequestID: c.GetString("request_id"),
		})
		return
	}

	c.Header("Content-Type", "image/png")
	c.Header("Content-Disposition", "inline; filename=\"qrcode.png\"")
	c.Header("Cache-Control", "public, max-age=86400")
	c.Data(http.StatusOK, "image/png", pngData)
}

// DownloadQRCode godoc
// @Summary Download QR code as file
// @Description Generate a QR code and return it as a downloadable file attachment.
// @Tags QR Code
// @Accept json
// @Produce image/png
// @Param filename query string false "Filename for download" default(qrcode.png)
// @Param request body models.GenerateQRRequest true "QR Code generation request"
// @Success 200 {file} binary "QR code PNG file"
// @Failure 400 {object} models.ErrorResponse "Invalid request"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/qr/download [post]
func (h *QRHandlers) DownloadQRCode(c *gin.Context) {
	var req models.GenerateQRRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:     fmt.Sprintf("Invalid request: %v", err),
			Code:      "INVALID_REQUEST",
			RequestID: c.GetString("request_id"),
		})
		return
	}

	size := 512
	if req.Size > 0 {
		size = req.Size
	}

	quality := req.Quality
	if quality == "" {
		quality = models.QualityHigh
	}

	filename := c.DefaultQuery("filename", "qrcode.png")

	pngData, err := h.qrService.GenerateQRCodePNG(c.Request.Context(), req.Type, &req.Data, size, quality)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:     fmt.Sprintf("Failed to generate QR code: %v", err),
			Code:      "GENERATION_FAILED",
			RequestID: c.GetString("request_id"),
		})
		return
	}

	c.Header("Content-Type", "image/png")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Header("Content-Length", strconv.Itoa(len(pngData)))
	c.Data(http.StatusOK, "image/png", pngData)
}

// BatchGenerateQRCodes godoc
// @Summary Batch generate QR codes
// @Description Generate multiple QR codes in a single request. Maximum 50 items per batch.
// @Tags QR Code
// @Accept json
// @Produce json
// @Param request body models.BatchGenerateRequest true "Batch generation request"
// @Success 200 {object} models.BatchGenerateResponse "Batch generation results"
// @Failure 400 {object} models.ErrorResponse "Invalid request"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/qr/batch [post]
func (h *QRHandlers) BatchGenerateQRCodes(c *gin.Context) {
	var req models.BatchGenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:     fmt.Sprintf("Invalid request: %v", err),
			Code:      "INVALID_REQUEST",
			RequestID: c.GetString("request_id"),
		})
		return
	}

	response := h.qrService.BatchGenerateQRCodes(c.Request.Context(), &req)
	c.JSON(http.StatusOK, response)
}

// GetQRTypes godoc
// @Summary List supported QR code types
// @Description Get a list of all supported QR code types with their required fields.
// @Tags QR Code
// @Produce json
// @Success 200 {object} map[string][]models.QRTypeInfo "List of supported QR types"
// @Router /api/v1/qr/types [get]
func (h *QRHandlers) GetQRTypes(c *gin.Context) {
	types := []map[string]interface{}{
		{
			"type":        "url",
			"name":        "URL",
			"description": "Website or web app URL",
			"fields":      []string{"url"},
			"example": map[string]interface{}{
				"type": "url",
				"data": map[string]string{"url": "https://example.com"},
			},
		},
		{
			"type":        "text",
			"name":        "Plain Text",
			"description": "Any text content",
			"fields":      []string{"text"},
			"example": map[string]interface{}{
				"type": "text",
				"data": map[string]string{"text": "Hello, World!"},
			},
		},
		{
			"type":        "wifi",
			"name":        "WiFi Network",
			"description": "WiFi network credentials for easy connection",
			"fields":      []string{"ssid", "password", "encryption", "hidden"},
			"example": map[string]interface{}{
				"type": "wifi",
				"data": map[string]interface{}{
					"wifi": map[string]interface{}{
						"ssid":       "MyNetwork",
						"password":   "secretpass123",
						"encryption": "WPA",
					},
				},
			},
		},
		{
			"type":        "vcard",
			"name":        "Contact (vCard)",
			"description": "Contact information that can be saved directly to phone",
			"fields":      []string{"first_name", "last_name", "email", "phone", "mobile", "organization", "title", "address", "website"},
			"example": map[string]interface{}{
				"type": "vcard",
				"data": map[string]interface{}{
					"vcard": map[string]string{
						"first_name":   "John",
						"last_name":    "Doe",
						"email":        "john@example.com",
						"phone":        "+1234567890",
						"organization": "Acme Inc",
					},
				},
			},
		},
		{
			"type":        "email",
			"name":        "Email",
			"description": "Email with optional subject and body pre-filled",
			"fields":      []string{"address", "subject", "body"},
			"example": map[string]interface{}{
				"type": "email",
				"data": map[string]interface{}{
					"email": map[string]string{
						"address": "contact@example.com",
						"subject": "Hello",
						"body":    "I wanted to reach out...",
					},
				},
			},
		},
		{
			"type":        "phone",
			"name":        "Phone Call",
			"description": "Phone number for one-tap calling",
			"fields":      []string{"phone"},
			"example": map[string]interface{}{
				"type": "phone",
				"data": map[string]string{"phone": "+1234567890"},
			},
		},
		{
			"type":        "sms",
			"name":        "SMS Message",
			"description": "SMS with optional pre-filled message",
			"fields":      []string{"phone", "message"},
			"example": map[string]interface{}{
				"type": "sms",
				"data": map[string]interface{}{
					"sms": map[string]string{
						"phone":   "+1234567890",
						"message": "Hello from QR!",
					},
				},
			},
		},
		{
			"type":        "geo",
			"name":        "Location",
			"description": "Geographic coordinates for maps",
			"fields":      []string{"latitude", "longitude", "altitude"},
			"example": map[string]interface{}{
				"type": "geo",
				"data": map[string]interface{}{
					"geo": map[string]float64{
						"latitude":  40.7128,
						"longitude": -74.0060,
					},
				},
			},
		},
		{
			"type":        "app",
			"name":        "App Download",
			"description": "App store links for iOS and Android apps",
			"fields":      []string{"ios_url", "android_url", "fallback_url"},
			"example": map[string]interface{}{
				"type": "app",
				"data": map[string]interface{}{
					"app": map[string]string{
						"ios_url":      "https://apps.apple.com/app/id123456",
						"android_url":  "https://play.google.com/store/apps/details?id=com.example",
						"fallback_url": "https://example.com/download",
					},
				},
			},
		},
		{
			"type":        "payment",
			"name":        "Payment",
			"description": "Payment QR codes (UPI, Bitcoin, Ethereum)",
			"fields":      []string{"type", "upi_id", "address", "amount", "currency", "name", "reference"},
			"example": map[string]interface{}{
				"type": "payment",
				"data": map[string]interface{}{
					"payment": map[string]interface{}{
						"type":     "upi",
						"upi_id":   "merchant@upi",
						"name":     "Store Name",
						"amount":   100.00,
						"currency": "INR",
					},
				},
			},
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"types": types,
		"total": len(types),
	})
}

// UploadQRCode godoc
// @Summary Upload a composite QR code with logo
// @Description Upload a QR code image (with optional logo overlay) to GCS storage
// @Tags QR Code
// @Accept json
// @Produce json
// @Param X-Tenant-ID header string true "Tenant ID"
// @Param request body models.UploadQRRequest true "Upload QR request with base64 image"
// @Success 200 {object} models.UploadQRResponse "QR code uploaded successfully"
// @Failure 400 {object} models.ErrorResponse "Invalid request"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/qr/upload [post]
func (h *QRHandlers) UploadQRCode(c *gin.Context) {
	var req models.UploadQRRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:     fmt.Sprintf("Invalid request: %v", err),
			Code:      "INVALID_REQUEST",
			RequestID: c.GetString("request_id"),
		})
		return
	}

	tenantID := c.GetHeader("X-Tenant-ID")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:     "X-Tenant-ID header is required",
			Code:      "MISSING_TENANT_ID",
			RequestID: c.GetString("request_id"),
		})
		return
	}

	if h.storageService == nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:     "Storage service not available",
			Code:      "STORAGE_UNAVAILABLE",
			RequestID: c.GetString("request_id"),
		})
		return
	}

	// Decode the base64 image
	imageData, err := base64.StdEncoding.DecodeString(req.ImageBase64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:     fmt.Sprintf("Invalid base64 image: %v", err),
			Code:      "INVALID_IMAGE",
			RequestID: c.GetString("request_id"),
		})
		return
	}

	// Upload the composite QR image
	contentType := req.ContentType
	if contentType == "" {
		contentType = "image/png"
	}

	objectName := fmt.Sprintf("%s/%s.png", tenantID, req.QRID)
	storageURL, err := h.storageService.Upload(c.Request.Context(), objectName, imageData, contentType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:     fmt.Sprintf("Failed to upload QR image: %v", err),
			Code:      "UPLOAD_FAILED",
			RequestID: c.GetString("request_id"),
		})
		return
	}

	response := models.UploadQRResponse{
		ID:         req.QRID,
		StorageURL: storageURL,
	}

	// If logo is provided, store it separately
	if req.LogoBase64 != "" {
		logoData, err := base64.StdEncoding.DecodeString(req.LogoBase64)
		if err == nil {
			logoObjectName := fmt.Sprintf("%s/%s_logo.png", tenantID, req.QRID)
			logoURL, err := h.storageService.Upload(c.Request.Context(), logoObjectName, logoData, "image/png")
			if err == nil {
				response.LogoURL = logoURL
			}
		}
	}

	c.JSON(http.StatusOK, response)
}
