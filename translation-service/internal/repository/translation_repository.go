package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/tesseract-hub/translation-service/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TranslationRepository interface for translation data operations
type TranslationRepository interface {
	// Language operations
	GetLanguages(ctx context.Context) ([]models.Language, error)
	GetLanguageByCode(ctx context.Context, code string) (*models.Language, error)
	UpsertLanguage(ctx context.Context, lang *models.Language) error
	SeedLanguages(ctx context.Context) error

	// Translation cache operations (database-backed)
	GetCachedTranslation(ctx context.Context, tenantID, sourceLang, targetLang, sourceHash string) (*models.TranslationCache, error)
	SaveTranslation(ctx context.Context, cache *models.TranslationCache) error
	IncrementHitCount(ctx context.Context, id uuid.UUID) error
	DeleteExpiredTranslations(ctx context.Context) (int64, error)
	DeleteTranslationsByTenant(ctx context.Context, tenantID string) error

	// Stats operations
	GetStats(ctx context.Context, tenantID string) (*models.TranslationStats, error)
	UpdateStats(ctx context.Context, tenantID string, cacheHit bool, characters int64) error

	// Tenant Preference operations
	GetPreference(ctx context.Context, tenantID string) (*models.TenantLanguagePreference, error)
	SavePreference(ctx context.Context, pref *models.TenantLanguagePreference) error

	// User Preference operations
	GetUserPreference(ctx context.Context, tenantID string, userID uuid.UUID) (*models.UserLanguagePreference, error)
	SaveUserPreference(ctx context.Context, pref *models.UserLanguagePreference) error
	DeleteUserPreference(ctx context.Context, tenantID string, userID uuid.UUID) error
}

// translationRepository implements TranslationRepository
type translationRepository struct {
	db *gorm.DB
}

// NewTranslationRepository creates a new translation repository
func NewTranslationRepository(db *gorm.DB) TranslationRepository {
	return &translationRepository{db: db}
}

// GetLanguages returns all active languages
func (r *translationRepository) GetLanguages(ctx context.Context) ([]models.Language, error) {
	var languages []models.Language
	err := r.db.WithContext(ctx).
		Where("is_active = ?", true).
		Order("region, name").
		Find(&languages).Error
	return languages, err
}

// GetLanguageByCode returns a language by its code
func (r *translationRepository) GetLanguageByCode(ctx context.Context, code string) (*models.Language, error) {
	var language models.Language
	err := r.db.WithContext(ctx).
		Where("code = ?", code).
		First(&language).Error
	if err != nil {
		return nil, err
	}
	return &language, nil
}

// UpsertLanguage inserts or updates a language
func (r *translationRepository) UpsertLanguage(ctx context.Context, lang *models.Language) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "code"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "native_name", "rtl", "is_active", "region", "updated_at"}),
		}).
		Create(lang).Error
}

// SeedLanguages seeds the initial languages
func (r *translationRepository) SeedLanguages(ctx context.Context) error {
	for _, lang := range models.SupportedLanguages {
		lang.IsActive = true
		if err := r.UpsertLanguage(ctx, &lang); err != nil {
			return err
		}
	}
	return nil
}

// GetCachedTranslation retrieves a cached translation from the database
func (r *translationRepository) GetCachedTranslation(ctx context.Context, tenantID, sourceLang, targetLang, sourceHash string) (*models.TranslationCache, error) {
	var cache models.TranslationCache
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND source_lang = ? AND target_lang = ? AND source_hash = ?",
			tenantID, sourceLang, targetLang, sourceHash).
		Where("expires_at > ?", time.Now()).
		First(&cache).Error
	if err != nil {
		return nil, err
	}
	return &cache, nil
}

// SaveTranslation saves a translation to the database cache
func (r *translationRepository) SaveTranslation(ctx context.Context, cache *models.TranslationCache) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "source_hash"}},
			DoUpdates: clause.AssignmentColumns([]string{"translated_text", "hit_count", "updated_at", "expires_at"}),
		}).
		Create(cache).Error
}

// IncrementHitCount increments the hit count for a cached translation
func (r *translationRepository) IncrementHitCount(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&models.TranslationCache{}).
		Where("id = ?", id).
		Update("hit_count", gorm.Expr("hit_count + 1")).Error
}

// DeleteExpiredTranslations removes expired translations from the database
func (r *translationRepository) DeleteExpiredTranslations(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("expires_at < ?", time.Now()).
		Delete(&models.TranslationCache{})
	return result.RowsAffected, result.Error
}

// DeleteTranslationsByTenant removes all translations for a tenant
func (r *translationRepository) DeleteTranslationsByTenant(ctx context.Context, tenantID string) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Delete(&models.TranslationCache{}).Error
}

// GetStats returns translation statistics for a tenant
func (r *translationRepository) GetStats(ctx context.Context, tenantID string) (*models.TranslationStats, error) {
	var stats models.TranslationStats
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		First(&stats).Error
	if err == gorm.ErrRecordNotFound {
		// Create new stats entry
		stats = models.TranslationStats{
			TenantID: tenantID,
		}
		if err := r.db.WithContext(ctx).Create(&stats).Error; err != nil {
			return nil, err
		}
		return &stats, nil
	}
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

// UpdateStats updates translation statistics
func (r *translationRepository) UpdateStats(ctx context.Context, tenantID string, cacheHit bool, characters int64) error {
	updates := map[string]interface{}{
		"total_requests":   gorm.Expr("total_requests + 1"),
		"total_characters": gorm.Expr("total_characters + ?", characters),
		"last_request_at":  time.Now(),
		"updated_at":       time.Now(),
	}

	if cacheHit {
		updates["cache_hits"] = gorm.Expr("cache_hits + 1")
	} else {
		updates["cache_misses"] = gorm.Expr("cache_misses + 1")
	}

	result := r.db.WithContext(ctx).
		Model(&models.TranslationStats{}).
		Where("tenant_id = ?", tenantID).
		Updates(updates)

	if result.RowsAffected == 0 {
		// Create new stats entry
		stats := models.TranslationStats{
			TenantID:        tenantID,
			TotalRequests:   1,
			TotalCharacters: characters,
			LastRequestAt:   time.Now(),
		}
		if cacheHit {
			stats.CacheHits = 1
		} else {
			stats.CacheMisses = 1
		}
		return r.db.WithContext(ctx).Create(&stats).Error
	}

	return result.Error
}

// GetPreference returns language preferences for a tenant
func (r *translationRepository) GetPreference(ctx context.Context, tenantID string) (*models.TenantLanguagePreference, error) {
	var pref models.TenantLanguagePreference
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		First(&pref).Error
	if err == gorm.ErrRecordNotFound {
		// Return default preference
		defaultEnabled, _ := json.Marshal([]string{"en", "hi", "ta", "te", "mr", "bn"})
		return &models.TenantLanguagePreference{
			TenantID:          tenantID,
			DefaultSourceLang: "en",
			DefaultTargetLang: "hi",
			EnabledLanguages:  defaultEnabled,
			AutoDetect:        true,
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return &pref, nil
}

// SavePreference saves language preferences for a tenant
func (r *translationRepository) SavePreference(ctx context.Context, pref *models.TenantLanguagePreference) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "tenant_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"default_source_lang", "default_target_lang", "enabled_languages", "auto_detect", "updated_at"}),
		}).
		Create(pref).Error
}

// GetUserPreference returns language preferences for a specific user within a tenant
// If no preference exists, returns default preference (English)
func (r *translationRepository) GetUserPreference(ctx context.Context, tenantID string, userID uuid.UUID) (*models.UserLanguagePreference, error) {
	var pref models.UserLanguagePreference
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		First(&pref).Error
	if err == gorm.ErrRecordNotFound {
		// Return default preference (English) - not persisted until user explicitly sets it
		return models.GetDefaultUserPreference(tenantID, userID), nil
	}
	if err != nil {
		return nil, err
	}
	return &pref, nil
}

// SaveUserPreference saves or updates language preference for a user within a tenant
func (r *translationRepository) SaveUserPreference(ctx context.Context, pref *models.UserLanguagePreference) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"preferred_language", "source_language", "auto_detect_source", "rtl_enabled", "updated_at"}),
		}).
		Create(pref).Error
}

// DeleteUserPreference removes language preference for a user (resets to default)
func (r *translationRepository) DeleteUserPreference(ctx context.Context, tenantID string, userID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Delete(&models.UserLanguagePreference{}).Error
}
