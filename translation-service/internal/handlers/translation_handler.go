package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"translation-service/internal/cache"
	"translation-service/internal/clients"
	"translation-service/internal/config"
	"translation-service/internal/middleware"
	"translation-service/internal/models"
	"translation-service/internal/repository"
)

// TranslationHandler handles translation API requests
type TranslationHandler struct {
	repo         repository.TranslationRepository
	cache        *cache.TranslationCache
	orchestrator *clients.TranslationOrchestrator
	// Keep legacy references for health checks and language detection
	libreTranslate *clients.LibreTranslateClient
	config         *config.TranslationConfig
	logger         *logrus.Entry
}

// normalizeLanguageCode converts common language code variants to LibreTranslate compatible codes
// This ensures consistent language code handling across different frontends and providers
func normalizeLanguageCode(code string) string {
	// Map frontend language codes to LibreTranslate expected codes
	languageCodeMap := map[string]string{
		"zh":      "zh-Hans", // Chinese Simplified
		"zh-CN":   "zh-Hans",
		"zh-TW":   "zh-Hans", // LibreTranslate only supports Simplified Chinese
		"zh-Hant": "zh-Hans",
	}

	if normalized, ok := languageCodeMap[code]; ok {
		return normalized
	}
	return code
}

// NewTranslationHandler creates a new translation handler with orchestrator
func NewTranslationHandler(
	repo repository.TranslationRepository,
	cache *cache.TranslationCache,
	orchestrator *clients.TranslationOrchestrator,
	libreTranslate *clients.LibreTranslateClient,
	cfg *config.TranslationConfig,
	logger *logrus.Entry,
) *TranslationHandler {
	return &TranslationHandler{
		repo:           repo,
		cache:          cache,
		orchestrator:   orchestrator,
		libreTranslate: libreTranslate,
		config:         cfg,
		logger:         logger,
	}
}

// Translate handles single translation requests
// POST /api/v1/translate
func (h *TranslationHandler) Translate(c *gin.Context) {
	var req models.TranslationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_REQUEST",
			"message": err.Error(),
		})
		return
	}

	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		tenantID = "default"
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// Set default source language if not provided
	sourceLang := req.SourceLang
	if sourceLang == "" {
		sourceLang = h.config.DefaultSourceLang
	}

	// Normalize language codes for provider compatibility
	sourceLang = normalizeLanguageCode(sourceLang)
	targetLang := normalizeLanguageCode(req.TargetLang)

	// Check Redis cache first
	if h.cache != nil && h.config.CacheEnabled {
		cached, err := h.cache.Get(ctx, tenantID, sourceLang, targetLang, req.Text, req.Context)
		if err == nil && cached != nil {
			h.logger.WithFields(logrus.Fields{
				"tenant_id":   tenantID,
				"source_lang": sourceLang,
				"target_lang": targetLang,
			}).Debug("Cache hit")

			// Update stats
			go h.repo.UpdateStats(context.Background(), tenantID, true, int64(len(req.Text)))

			c.JSON(http.StatusOK, models.TranslationResponse{
				OriginalText:   req.Text,
				TranslatedText: cached.TranslatedText,
				SourceLang:     sourceLang,
				TargetLang:     targetLang,
				Cached:         true,
				Provider:       cached.Provider,
			})
			return
		}
	}

	// Check database cache
	sourceHash := models.GenerateSourceHash(sourceLang, targetLang, req.Text, req.Context)
	dbCached, err := h.repo.GetCachedTranslation(ctx, tenantID, sourceLang, targetLang, sourceHash)
	if err == nil && dbCached != nil {
		h.logger.WithFields(logrus.Fields{
			"tenant_id":   tenantID,
			"source_lang": sourceLang,
			"target_lang": targetLang,
		}).Debug("Database cache hit")

		// Update Redis cache
		if h.cache != nil && h.config.CacheEnabled {
			go h.cache.Set(context.Background(), tenantID, sourceLang, targetLang, req.Text, dbCached.TranslatedText, req.Context, dbCached.Provider)
		}

		// Update hit count
		go h.repo.IncrementHitCount(context.Background(), dbCached.ID)
		go h.repo.UpdateStats(context.Background(), tenantID, true, int64(len(req.Text)))

		c.JSON(http.StatusOK, models.TranslationResponse{
			OriginalText:   req.Text,
			TranslatedText: dbCached.TranslatedText,
			SourceLang:     sourceLang,
			TargetLang:     targetLang,
			Cached:         true,
			Provider:       dbCached.Provider,
		})
		return
	}

	// Translate using the orchestrator (tries providers in priority order)
	result, err := h.orchestrator.Translate(ctx, req.Text, sourceLang, targetLang)
	if err != nil {
		h.logger.WithFields(logrus.Fields{
			"error":       err.Error(),
			"source_lang": sourceLang,
			"target_lang": targetLang,
		}).Error("Translation failed (all providers)")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "TRANSLATION_FAILED",
			"message": "Failed to translate text",
		})
		return
	}

	translated := result.TranslatedText
	provider := string(result.Provider)

	// Cache the result
	cacheEntry := &models.TranslationCache{
		TenantID:       tenantID,
		SourceLang:     sourceLang,
		TargetLang:     targetLang,
		SourceHash:     sourceHash,
		SourceText:     req.Text,
		TranslatedText: translated,
		Context:        req.Context,
		Provider:       provider,
		ExpiresAt:      time.Now().Add(h.config.CacheTTL),
	}

	// Save to database cache
	go func() {
		if err := h.repo.SaveTranslation(context.Background(), cacheEntry); err != nil {
			h.logger.WithError(err).Warn("Failed to save translation to database cache")
		}
	}()

	// Save to Redis cache
	if h.cache != nil && h.config.CacheEnabled {
		go h.cache.Set(context.Background(), tenantID, sourceLang, targetLang, req.Text, translated, req.Context, provider)
	}

	// Update stats
	go h.repo.UpdateStats(context.Background(), tenantID, false, int64(len(req.Text)))

	c.JSON(http.StatusOK, models.TranslationResponse{
		OriginalText:   req.Text,
		TranslatedText: translated,
		SourceLang:     sourceLang,
		TargetLang:     targetLang,
		Cached:         false,
		Provider:       provider,
	})
}

// TranslateBatch handles batch translation requests
// POST /api/v1/translate/batch
func (h *TranslationHandler) TranslateBatch(c *gin.Context) {
	var req models.BatchTranslationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_REQUEST",
			"message": err.Error(),
		})
		return
	}

	if len(req.Items) > h.config.MaxBatchSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "BATCH_TOO_LARGE",
			"message": "Batch size exceeds maximum allowed",
			"max_size": h.config.MaxBatchSize,
		})
		return
	}

	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		tenantID = "default"
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), h.config.BatchTimeout)
	defer cancel()

	// Normalize target language code for provider compatibility
	targetLang := normalizeLanguageCode(req.TargetLang)

	response := models.BatchTranslationResponse{
		Items:      make([]models.BatchTranslationItem, len(req.Items)),
		TotalCount: len(req.Items),
		TargetLang: targetLang,
	}

	for i, item := range req.Items {
		sourceLang := item.SourceLang
		if sourceLang == "" {
			sourceLang = req.SourceLang
		}
		if sourceLang == "" {
			sourceLang = h.config.DefaultSourceLang
		}
		// Normalize source language code
		sourceLang = normalizeLanguageCode(sourceLang)

		responseItem := models.BatchTranslationItem{
			ID:           item.ID,
			OriginalText: item.Text,
			SourceLang:   sourceLang,
		}

		// Check Redis cache first
		if h.cache != nil && h.config.CacheEnabled {
			cached, err := h.cache.Get(ctx, tenantID, sourceLang, targetLang, item.Text, item.Context)
			if err == nil && cached != nil {
				responseItem.TranslatedText = cached.TranslatedText
				responseItem.Cached = true
				response.CachedCount++
				response.Items[i] = responseItem
				continue
			}
		}

		// Check database cache
		sourceHash := models.GenerateSourceHash(sourceLang, targetLang, item.Text, item.Context)
		dbCached, err := h.repo.GetCachedTranslation(ctx, tenantID, sourceLang, targetLang, sourceHash)
		if err == nil && dbCached != nil {
			responseItem.TranslatedText = dbCached.TranslatedText
			responseItem.Cached = true
			response.CachedCount++
			response.Items[i] = responseItem

			// Update Redis cache
			if h.cache != nil && h.config.CacheEnabled {
				go h.cache.Set(context.Background(), tenantID, sourceLang, targetLang, item.Text, dbCached.TranslatedText, item.Context, dbCached.Provider)
			}
			continue
		}

		// Translate using orchestrator
		result, err := h.orchestrator.Translate(ctx, item.Text, sourceLang, targetLang)
		if err != nil {
			responseItem.Error = err.Error()
			responseItem.TranslatedText = item.Text // Return original on error
		} else {
			responseItem.TranslatedText = result.TranslatedText

			// Cache the result
			cacheEntry := &models.TranslationCache{
				TenantID:       tenantID,
				SourceLang:     sourceLang,
				TargetLang:     targetLang,
				SourceHash:     sourceHash,
				SourceText:     item.Text,
				TranslatedText: result.TranslatedText,
				Context:        item.Context,
				Provider:       string(result.Provider),
				ExpiresAt:      time.Now().Add(h.config.CacheTTL),
			}

			go func(entry *models.TranslationCache) {
				if err := h.repo.SaveTranslation(context.Background(), entry); err != nil {
					h.logger.WithError(err).Warn("Failed to save batch translation to cache")
				}
			}(cacheEntry)

			if h.cache != nil && h.config.CacheEnabled {
				go h.cache.Set(context.Background(), tenantID, sourceLang, targetLang, item.Text, result.TranslatedText, item.Context, string(result.Provider))
			}
		}

		response.Items[i] = responseItem
	}

	// Update stats
	totalChars := int64(0)
	for _, item := range req.Items {
		totalChars += int64(len(item.Text))
	}
	go h.repo.UpdateStats(context.Background(), tenantID, response.CachedCount > 0, totalChars)

	c.JSON(http.StatusOK, response)
}

// DetectLanguage handles language detection requests
// POST /api/v1/detect
func (h *TranslationHandler) DetectLanguage(c *gin.Context) {
	var req models.DetectLanguageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_REQUEST",
			"message": err.Error(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	result, err := h.libreTranslate.DetectLanguage(ctx, req.Text)
	if err != nil {
		h.logger.WithError(err).Error("Language detection failed")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "DETECTION_FAILED",
			"message": "Failed to detect language",
		})
		return
	}

	c.JSON(http.StatusOK, models.DetectLanguageResponse{
		Language:   result.Language,
		Confidence: result.Confidence,
	})
}

// GetLanguages returns supported languages
// GET /api/v1/languages
func (h *TranslationHandler) GetLanguages(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	languages, err := h.repo.GetLanguages(ctx)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get languages")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "LANGUAGES_FAILED",
			"message": "Failed to retrieve languages",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"languages": languages,
		"count":     len(languages),
	})
}

// GetStats returns translation statistics for a tenant
// GET /api/v1/stats
func (h *TranslationHandler) GetStats(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		tenantID = "default"
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	stats, err := h.repo.GetStats(ctx, tenantID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get stats")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "STATS_FAILED",
			"message": "Failed to retrieve statistics",
		})
		return
	}

	cacheHitRate := float64(0)
	if stats.TotalRequests > 0 {
		cacheHitRate = float64(stats.CacheHits) / float64(stats.TotalRequests) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"tenant_id":        stats.TenantID,
		"total_requests":   stats.TotalRequests,
		"cache_hits":       stats.CacheHits,
		"cache_misses":     stats.CacheMisses,
		"cache_hit_rate":   cacheHitRate,
		"total_characters": stats.TotalCharacters,
		"last_request_at":  stats.LastRequestAt,
	})
}

// GetPreference returns language preferences for a tenant
// GET /api/v1/preferences
func (h *TranslationHandler) GetPreference(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "MISSING_TENANT_ID",
			"message": "X-Tenant-ID header is required",
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	pref, err := h.repo.GetPreference(ctx, tenantID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get preferences")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "PREFERENCE_FAILED",
			"message": "Failed to retrieve preferences",
		})
		return
	}

	var enabledLanguages []string
	if len(pref.EnabledLanguages) > 0 {
		json.Unmarshal(pref.EnabledLanguages, &enabledLanguages)
	}

	c.JSON(http.StatusOK, gin.H{
		"tenant_id":           pref.TenantID,
		"default_source_lang": pref.DefaultSourceLang,
		"default_target_lang": pref.DefaultTargetLang,
		"enabled_languages":   enabledLanguages,
		"auto_detect":         pref.AutoDetect,
	})
}

// UpdatePreference updates language preferences for a tenant
// PUT /api/v1/preferences
func (h *TranslationHandler) UpdatePreference(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "MISSING_TENANT_ID",
			"message": "X-Tenant-ID header is required",
		})
		return
	}

	var req struct {
		DefaultSourceLang string   `json:"default_source_lang"`
		DefaultTargetLang string   `json:"default_target_lang"`
		EnabledLanguages  []string `json:"enabled_languages"`
		AutoDetect        bool     `json:"auto_detect"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_REQUEST",
			"message": err.Error(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	enabledLanguagesJSON, _ := json.Marshal(req.EnabledLanguages)

	pref := &models.TenantLanguagePreference{
		TenantID:          tenantID,
		DefaultSourceLang: req.DefaultSourceLang,
		DefaultTargetLang: req.DefaultTargetLang,
		EnabledLanguages:  enabledLanguagesJSON,
		AutoDetect:        req.AutoDetect,
	}

	if err := h.repo.SavePreference(ctx, pref); err != nil {
		h.logger.WithError(err).Error("Failed to save preferences")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "PREFERENCE_SAVE_FAILED",
			"message": "Failed to save preferences",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Preferences updated successfully",
	})
}

// InvalidateCache invalidates translation cache for a tenant
// DELETE /api/v1/cache
func (h *TranslationHandler) InvalidateCache(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "MISSING_TENANT_ID",
			"message": "X-Tenant-ID header is required",
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// Invalidate Redis cache
	if h.cache != nil {
		if err := h.cache.InvalidateTenant(ctx, tenantID); err != nil {
			h.logger.WithError(err).Warn("Failed to invalidate Redis cache")
		}
	}

	// Invalidate database cache
	if err := h.repo.DeleteTranslationsByTenant(ctx, tenantID); err != nil {
		h.logger.WithError(err).Error("Failed to invalidate database cache")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "CACHE_INVALIDATION_FAILED",
			"message": "Failed to invalidate cache",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Cache invalidated successfully",
	})
}

// Health returns service health status
// GET /health
func (h *TranslationHandler) Health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	status := "healthy"
	checks := make(map[string]string)

	// Check all translation providers via orchestrator
	providerHealth := h.orchestrator.GetProviderHealth()
	healthyProviders := 0
	for name, health := range providerHealth {
		if health.Healthy {
			checks[string(name)] = "healthy"
			healthyProviders++
		} else {
			checks[string(name)] = "unhealthy: " + health.LastError
		}
	}

	// Service is degraded if no providers are healthy
	if healthyProviders == 0 {
		status = "unhealthy"
	} else if healthyProviders < len(providerHealth) {
		status = "degraded"
	}

	// Check Redis
	if h.cache != nil {
		if err := h.cache.HealthCheck(ctx); err != nil {
			checks["redis"] = "unhealthy: " + err.Error()
			if status == "healthy" {
				status = "degraded"
			}
		} else {
			checks["redis"] = "healthy"
		}
	}

	// Add provider metrics
	metrics := h.orchestrator.GetProviderMetrics()
	metricsMap := make(map[string]interface{})
	for name, m := range metrics {
		metricsMap[string(name)] = map[string]interface{}{
			"total_requests": m.TotalRequests,
			"success_count":  m.SuccessfulCount,
			"failed_count":   m.FailedCount,
			"characters":     m.CharactersCount,
		}
	}

	statusCode := http.StatusOK
	if status == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, gin.H{
		"status":           status,
		"checks":           checks,
		"provider_metrics": metricsMap,
		"provider_chain":   h.orchestrator.GetProviders(),
	})
}

// Livez returns liveness status
// GET /livez
func (h *TranslationHandler) Livez(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "alive"})
}

// Readyz returns readiness status
// GET /readyz
func (h *TranslationHandler) Readyz(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// Check if at least one provider is healthy
	providerHealth := h.orchestrator.GetProviderHealth()
	for _, health := range providerHealth {
		if health.Healthy {
			c.JSON(http.StatusOK, gin.H{"status": "ready"})
			return
		}
	}

	// Fall back to checking LibreTranslate directly
	if h.libreTranslate != nil {
		if err := h.libreTranslate.HealthCheck(ctx); err == nil {
			c.JSON(http.StatusOK, gin.H{"status": "ready"})
			return
		}
	}

	c.JSON(http.StatusServiceUnavailable, gin.H{
		"status": "not ready",
		"error":  "no healthy translation providers",
	})
}

// GetUserLanguagePreference returns language preference for the current user
// GET /api/v1/users/me/language
// Default is English if user hasn't set a preference
func (h *TranslationHandler) GetUserLanguagePreference(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "MISSING_TENANT_ID",
			"message": "X-Tenant-ID header is required",
		})
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "MISSING_USER_ID",
			"message": "X-User-ID header is required",
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	pref, err := h.repo.GetUserPreference(ctx, tenantID, userID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get user language preference")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "PREFERENCE_FAILED",
			"message": "Failed to retrieve user language preference",
		})
		return
	}

	// Check if this is a default (not persisted) preference
	isDefault := pref.ID.String() == "00000000-0000-0000-0000-000000000000" || pref.CreatedAt.IsZero()

	c.JSON(http.StatusOK, models.UserPreferenceResponse{
		Success: true,
		Data:    pref,
		Message: func() string {
			if isDefault {
				return "Using default language (English). Set a preference to persist your choice."
			}
			return ""
		}(),
	})
}

// SetUserLanguagePreference sets or updates language preference for the current user
// PUT /api/v1/users/me/language
// Once set, this preference persists and becomes the default for this user
func (h *TranslationHandler) SetUserLanguagePreference(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "MISSING_TENANT_ID",
			"message": "X-Tenant-ID header is required",
		})
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "MISSING_USER_ID",
			"message": "X-User-ID header is required",
		})
		return
	}

	var req models.UserPreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_REQUEST",
			"message": err.Error(),
		})
		return
	}

	// Validate the preferred language exists
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	lang, err := h.repo.GetLanguageByCode(ctx, req.PreferredLanguage)
	if err != nil || lang == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_LANGUAGE",
			"message": "The specified language is not supported",
			"valid_languages": func() []string {
				langs, _ := h.repo.GetLanguages(ctx)
				codes := make([]string, len(langs))
				for i, l := range langs {
					codes[i] = l.Code
				}
				return codes
			}(),
		})
		return
	}

	// Get existing preference or create new one
	pref, _ := h.repo.GetUserPreference(ctx, tenantID, userID)
	if pref == nil {
		pref = models.GetDefaultUserPreference(tenantID, userID)
	}

	// Update fields
	pref.PreferredLanguage = req.PreferredLanguage
	pref.TenantID = tenantID
	pref.UserID = userID

	if req.SourceLanguage != "" {
		pref.SourceLanguage = req.SourceLanguage
	}
	if req.AutoDetectSource != nil {
		pref.AutoDetectSource = *req.AutoDetectSource
	}
	if req.RTLEnabled != nil {
		pref.RTLEnabled = *req.RTLEnabled
	}

	// Auto-set RTL based on language
	if lang.RTL {
		pref.RTLEnabled = true
	}

	if err := h.repo.SaveUserPreference(ctx, pref); err != nil {
		h.logger.WithError(err).Error("Failed to save user language preference")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "PREFERENCE_SAVE_FAILED",
			"message": "Failed to save user language preference",
		})
		return
	}

	h.logger.WithFields(logrus.Fields{
		"tenant_id":          tenantID,
		"user_id":            userID,
		"preferred_language": req.PreferredLanguage,
	}).Info("User language preference saved")

	c.JSON(http.StatusOK, models.UserPreferenceResponse{
		Success: true,
		Data:    pref,
		Message: "Language preference saved successfully",
	})
}

// ResetUserLanguagePreference resets user's language preference to default (English)
// DELETE /api/v1/users/me/language
func (h *TranslationHandler) ResetUserLanguagePreference(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "MISSING_TENANT_ID",
			"message": "X-Tenant-ID header is required",
		})
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "MISSING_USER_ID",
			"message": "X-User-ID header is required",
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if err := h.repo.DeleteUserPreference(ctx, tenantID, userID); err != nil {
		h.logger.WithError(err).Error("Failed to reset user language preference")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "PREFERENCE_RESET_FAILED",
			"message": "Failed to reset user language preference",
		})
		return
	}

	h.logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"user_id":   userID,
	}).Info("User language preference reset to default")

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Language preference reset to default (English)",
	})
}
