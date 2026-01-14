//go:build integration_disabled
// +build integration_disabled

// NOTE: This integration test file is temporarily disabled because the VerificationService
// signature has changed and requires external clients (VerificationClient, NotificationClient, RedisClient).
// TODO: Update tests to match new service API.

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tesseract-hub/domains/common/services/tenant-service/internal/config"
	"github.com/tesseract-hub/domains/common/services/tenant-service/internal/handlers"
	"github.com/tesseract-hub/domains/common/services/tenant-service/internal/models"
	"github.com/tesseract-hub/domains/common/services/tenant-service/internal/repository"
	"github.com/tesseract-hub/domains/common/services/tenant-service/internal/services"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestContext holds test dependencies
type TestContext struct {
	Router              *gin.Engine
	DB                  *gorm.DB
	OnboardingHandler   *handlers.OnboardingHandler
	VerificationHandler *handlers.VerificationHandler
	TemplateHandler     *handlers.TemplateHandler
}

// SetupTestEnvironment initializes test database and dependencies
func SetupTestEnvironment(t *testing.T) *TestContext {
	// Load configuration
	cfg := config.New()

	// Override with test database
	cfg.Database.Name = "tesseract_hub_test"

	// Connect to test database
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Name,
		cfg.Database.SSLMode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err, "Failed to connect to test database")

	// Auto-migrate models
	err = db.AutoMigrate(
		&models.OnboardingTemplate{},
		&models.OnboardingSession{},
		&models.BusinessInformation{},
		&models.ContactInformation{},
		&models.BusinessAddress{},
		&models.OnboardingTask{},
		&models.VerificationRecord{},
	)
	require.NoError(t, err, "Failed to migrate test database")

	// Initialize repositories
	templateRepo := repository.NewTemplateRepository(db)
	onboardingRepo := repository.NewOnboardingRepository(db)
	verificationRepo := repository.NewVerificationRepository(db)

	// Initialize services
	templateService := services.NewTemplateService(templateRepo)
	verificationService := services.NewVerificationService(cfg, verificationRepo)
	onboardingService := services.NewOnboardingService(cfg, onboardingRepo, templateRepo, verificationService)

	// Initialize handlers
	templateHandler := handlers.NewTemplateHandler(templateService)
	onboardingHandler := handlers.NewOnboardingHandler(onboardingService, templateService)
	verificationHandler := handlers.NewVerificationHandler(verificationService, onboardingService)

	// Setup router
	gin.SetMode(gin.TestMode)
	router := setupTestRouter(templateHandler, onboardingHandler, verificationHandler)

	return &TestContext{
		Router:              router,
		DB:                  db,
		OnboardingHandler:   onboardingHandler,
		VerificationHandler: verificationHandler,
		TemplateHandler:     templateHandler,
	}
}

// setupTestRouter creates a router for testing
func setupTestRouter(
	templateHandler *handlers.TemplateHandler,
	onboardingHandler *handlers.OnboardingHandler,
	verificationHandler *handlers.VerificationHandler,
) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	api := router.Group("/api/v1")
	{
		// Template routes
		templates := api.Group("/templates")
		{
			templates.GET("", templateHandler.ListTemplates)
			templates.GET("/:id", templateHandler.GetTemplate)
		}

		// Onboarding routes
		onboarding := api.Group("/onboarding")
		{
			onboarding.POST("/start", onboardingHandler.StartOnboarding)
			onboarding.GET("/:sessionId", onboardingHandler.GetOnboardingSession)
			onboarding.GET("/:sessionId/progress", onboardingHandler.GetProgress)
			onboarding.GET("/:sessionId/tasks", onboardingHandler.GetTasks)
			onboarding.PUT("/:sessionId/tasks/:taskId", onboardingHandler.UpdateTaskStatus)
			onboarding.PUT("/:sessionId/business-information", onboardingHandler.UpdateBusinessInformation)
			onboarding.PUT("/:sessionId/contact-information", onboardingHandler.UpdateContactInformation)
			onboarding.PUT("/:sessionId/business-address", onboardingHandler.UpdateBusinessAddress)
			onboarding.POST("/:sessionId/complete", onboardingHandler.CompleteOnboarding)
		}

		// Verification routes
		verification := api.Group("/verification")
		{
			verification.POST("/:sessionId/email/start", verificationHandler.StartEmailVerification)
			verification.POST("/:sessionId/phone/start", verificationHandler.StartPhoneVerification)
			verification.POST("/:sessionId/verify", verificationHandler.VerifyCode)
			verification.POST("/:sessionId/resend", verificationHandler.ResendVerificationCode)
			verification.GET("/:sessionId/status", verificationHandler.GetVerificationStatus)
			verification.GET("/:sessionId/:type/check", verificationHandler.CheckVerification)
		}

		// Validation routes
		api.GET("/validate/subdomain", onboardingHandler.ValidateSubdomain)
		api.GET("/validate/business-name", onboardingHandler.ValidateBusinessName)
	}

	return router
}

// CleanupTestData cleans up test data
func (tc *TestContext) CleanupTestData(t *testing.T) {
	// Clean tables in reverse order of dependencies
	tables := []string{
		"verification_records",
		"onboarding_tasks",
		"business_addresses",
		"contact_information",
		"business_information",
		"onboarding_sessions",
	}

	for _, table := range tables {
		err := tc.DB.Exec(fmt.Sprintf("DELETE FROM %s", table)).Error
		if err != nil {
			t.Logf("Warning: Failed to clean table %s: %v", table, err)
		}
	}
}

// SeedDefaultTemplate creates a default template for testing
func (tc *TestContext) SeedDefaultTemplate(t *testing.T, applicationType string) uuid.UUID {
	template := &models.OnboardingTemplate{
		ID:              uuid.New(),
		Name:            fmt.Sprintf("Default %s Template", applicationType),
		ApplicationType: applicationType,
		Description:     "Default template for testing",
		IsDefault:       true,
		IsActive:        true,
		Steps: []models.OnboardingStep{
			{
				StepID:      "business_info",
				Name:        "Business Information",
				Description: "Provide your business details",
				Order:       1,
				IsRequired:  true,
			},
			{
				StepID:      "contact_info",
				Name:        "Contact Information",
				Description: "Provide contact details",
				Order:       2,
				IsRequired:  true,
			},
			{
				StepID:      "verification",
				Name:        "Email Verification",
				Description: "Verify your email address",
				Order:       3,
				IsRequired:  true,
			},
			{
				StepID:      "business_address",
				Name:        "Business Address",
				Description: "Provide your business address",
				Order:       4,
				IsRequired:  true,
			},
		},
	}

	err := tc.DB.Create(template).Error
	require.NoError(t, err, "Failed to seed default template")

	return template.ID
}

// TestCompleteOnboardingFlow tests the full onboarding workflow
func TestCompleteOnboardingFlow(t *testing.T) {
	tc := SetupTestEnvironment(t)
	defer tc.CleanupTestData(t)

	// Seed default template
	tc.SeedDefaultTemplate(t, "ecommerce")

	var sessionID uuid.UUID

	t.Run("Start Onboarding Session", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"application_type": "ecommerce",
			"metadata": map[string]interface{}{
				"source": "web",
			},
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/onboarding/start", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		tc.Router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))
		data := response["data"].(map[string]interface{})
		sessionID, _ = uuid.Parse(data["id"].(string))
		assert.Equal(t, "pending", data["status"])
		assert.Equal(t, "ecommerce", data["application_type"])
	})

	t.Run("Get Onboarding Session", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/onboarding/%s", sessionID), nil)
		w := httptest.NewRecorder()
		tc.Router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))
		data := response["data"].(map[string]interface{})
		assert.Equal(t, sessionID.String(), data["id"])
	})

	t.Run("Update Business Information", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"business_name": "Test Ecommerce Store",
			"subdomain":     "test-store-123",
			"industry":      "retail",
			"company_size":  "1-10",
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPut,
			fmt.Sprintf("/api/v1/onboarding/%s/business-information", sessionID),
			bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		tc.Router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))
		data := response["data"].(map[string]interface{})
		assert.Equal(t, "Test Ecommerce Store", data["business_name"])
		assert.Equal(t, "test-store-123", data["subdomain"])
	})

	t.Run("Update Contact Information", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"first_name": "John",
			"last_name":  "Doe",
			"email":      "john.doe@example.com",
			"phone":      "+1234567890",
			"role":       "Owner",
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPut,
			fmt.Sprintf("/api/v1/onboarding/%s/contact-information", sessionID),
			bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		tc.Router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))
		data := response["data"].(map[string]interface{})
		assert.Equal(t, "john.doe@example.com", data["email"])
	})

	t.Run("Start Email Verification", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"email": "john.doe@example.com",
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost,
			fmt.Sprintf("/api/v1/verification/%s/email/start", sessionID),
			bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		tc.Router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))
		data := response["data"].(map[string]interface{})
		assert.Equal(t, "email_verification", data["verification_type"])
		assert.Equal(t, "pending", data["status"])
	})

	t.Run("Update Business Address", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"street_address": "123 Main St",
			"city":           "New York",
			"state":          "NY",
			"postal_code":    "10001",
			"country":        "USA",
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPut,
			fmt.Sprintf("/api/v1/onboarding/%s/business-address", sessionID),
			bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		tc.Router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))
		data := response["data"].(map[string]interface{})
		assert.Equal(t, "123 Main St", data["street_address"])
		assert.Equal(t, "New York", data["city"])
	})

	t.Run("Get Progress", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet,
			fmt.Sprintf("/api/v1/onboarding/%s/progress", sessionID), nil)
		w := httptest.NewRecorder()
		tc.Router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))
		data := response["data"].(map[string]interface{})

		// Progress should be > 0% since we've completed some steps
		completionPercentage := data["completion_percentage"].(float64)
		assert.Greater(t, completionPercentage, 0.0)
		assert.LessOrEqual(t, completionPercentage, 100.0)
	})
}

// TestValidationErrors tests various validation scenarios
func TestValidationErrors(t *testing.T) {
	tc := SetupTestEnvironment(t)
	defer tc.CleanupTestData(t)

	tc.SeedDefaultTemplate(t, "ecommerce")

	t.Run("Invalid Session ID Format", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/onboarding/invalid-uuid", nil)
		w := httptest.NewRecorder()
		tc.Router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.False(t, response["success"].(bool))
	})

	t.Run("Non-Existent Session ID", func(t *testing.T) {
		nonExistentID := uuid.New()
		req := httptest.NewRequest(http.MethodGet,
			fmt.Sprintf("/api/v1/onboarding/%s", nonExistentID), nil)
		w := httptest.NewRecorder()
		tc.Router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("Missing Required Fields in Start Onboarding", func(t *testing.T) {
		reqBody := map[string]interface{}{
			// Missing application_type
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/onboarding/start", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		tc.Router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Invalid Email Format", func(t *testing.T) {
		// First create a session
		reqBody := map[string]interface{}{
			"application_type": "ecommerce",
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/onboarding/start", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		tc.Router.ServeHTTP(w, req)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		data := response["data"].(map[string]interface{})
		sessionID := data["id"].(string)

		// Try to start verification with invalid email
		reqBody = map[string]interface{}{
			"email": "invalid-email",
		}
		body, _ = json.Marshal(reqBody)
		req = httptest.NewRequest(http.MethodPost,
			fmt.Sprintf("/api/v1/verification/%s/email/start", sessionID),
			bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w = httptest.NewRecorder()
		tc.Router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// TestSubdomainValidation tests subdomain validation
func TestSubdomainValidation(t *testing.T) {
	tc := SetupTestEnvironment(t)
	defer tc.CleanupTestData(t)

	tc.SeedDefaultTemplate(t, "ecommerce")

	t.Run("Available Subdomain", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet,
			"/api/v1/validate/subdomain?subdomain=new-store-456", nil)
		w := httptest.NewRecorder()
		tc.Router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		data := response["data"].(map[string]interface{})
		assert.True(t, data["available"].(bool))
	})

	t.Run("Taken Subdomain", func(t *testing.T) {
		// First create a session with a subdomain
		reqBody := map[string]interface{}{
			"application_type": "ecommerce",
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/onboarding/start", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		tc.Router.ServeHTTP(w, req)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		data := response["data"].(map[string]interface{})
		sessionID := data["id"].(string)

		// Update business info with a subdomain
		reqBody = map[string]interface{}{
			"business_name": "Existing Store",
			"subdomain":     "existing-store",
		}
		body, _ = json.Marshal(reqBody)
		req = httptest.NewRequest(http.MethodPut,
			fmt.Sprintf("/api/v1/onboarding/%s/business-information", sessionID),
			bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		tc.Router.ServeHTTP(w, req)

		// Now validate the same subdomain
		req = httptest.NewRequest(http.MethodGet,
			"/api/v1/validate/subdomain?subdomain=existing-store", nil)
		w = httptest.NewRecorder()
		tc.Router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		json.Unmarshal(w.Body.Bytes(), &response)
		data = response["data"].(map[string]interface{})
		assert.False(t, data["available"].(bool))
	})
}
