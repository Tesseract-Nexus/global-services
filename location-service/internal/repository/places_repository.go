package repository

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"location-service/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PlacesRepository interface for places database operations
type PlacesRepository interface {
	// Create adds a new place to the database
	Create(ctx context.Context, place *models.Place) error

	// GetByID retrieves a place by its UUID
	GetByID(ctx context.Context, id uuid.UUID) (*models.Place, error)

	// GetByExternalID retrieves a place by external provider ID
	GetByExternalID(ctx context.Context, externalID string) (*models.Place, error)

	// Search performs full-text search on places
	Search(ctx context.Context, query string, filters models.SearchFilters) ([]models.Place, int64, error)

	// FindNearby finds places within a radius of coordinates
	FindNearby(ctx context.Context, lat, lng, radiusKm float64, limit int) ([]models.Place, error)

	// GetByCity retrieves places in a specific city
	GetByCity(ctx context.Context, city, countryCode string, limit, offset int) ([]models.Place, int64, error)

	// GetByPostalCode retrieves places with a specific postal code
	GetByPostalCode(ctx context.Context, postalCode, countryCode string, limit, offset int) ([]models.Place, int64, error)

	// Update updates an existing place
	Update(ctx context.Context, place *models.Place) error

	// Delete soft-deletes a place
	Delete(ctx context.Context, id uuid.UUID) error

	// BulkUpsert performs batch insert/update of places
	BulkUpsert(ctx context.Context, places []models.Place) error

	// GetStats returns statistics about stored places
	GetStats(ctx context.Context) (*models.PlacesStats, error)

	// SetVerified marks a place as verified
	SetVerified(ctx context.Context, id uuid.UUID, verified bool) error
}

// placesRepository implements PlacesRepository
type placesRepository struct {
	db *gorm.DB
}

// NewPlacesRepository creates a new places repository
func NewPlacesRepository(db *gorm.DB) PlacesRepository {
	return &placesRepository{db: db}
}

// Create adds a new place to the database
func (r *placesRepository) Create(ctx context.Context, place *models.Place) error {
	if place.ID == uuid.Nil {
		place.ID = uuid.New()
	}
	return r.db.WithContext(ctx).Create(place).Error
}

// GetByID retrieves a place by its UUID
func (r *placesRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Place, error) {
	var place models.Place
	err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&place).Error
	if err != nil {
		return nil, err
	}
	return &place, nil
}

// GetByExternalID retrieves a place by external provider ID
func (r *placesRepository) GetByExternalID(ctx context.Context, externalID string) (*models.Place, error) {
	var place models.Place
	err := r.db.WithContext(ctx).
		Where("external_place_id = ? AND deleted_at IS NULL", externalID).
		First(&place).Error
	if err != nil {
		return nil, err
	}
	return &place, nil
}

// Search performs full-text search on places
func (r *placesRepository) Search(ctx context.Context, query string, filters models.SearchFilters) ([]models.Place, int64, error) {
	var places []models.Place
	var total int64

	db := r.db.WithContext(ctx).Model(&models.Place{}).Where("deleted_at IS NULL")

	// Sanitize query for tsquery (reused in where and order clauses)
	var sanitizedQuery string
	if query != "" {
		sanitizedQuery = sanitizeSearchQuery(query)
		db = db.Where("search_vector @@ plainto_tsquery('english', ?)", sanitizedQuery)
	}

	// Apply filters
	if filters.CountryCode != "" {
		db = db.Where("country_code = ?", strings.ToUpper(filters.CountryCode))
	}
	if filters.City != "" {
		db = db.Where("LOWER(city) = LOWER(?)", filters.City)
	}
	if filters.StateCode != "" {
		db = db.Where("state_code = ?", strings.ToUpper(filters.StateCode))
	}
	if filters.PostalCode != "" {
		db = db.Where("postal_code = ?", filters.PostalCode)
	}
	if filters.Verified != nil {
		db = db.Where("verified = ?", *filters.Verified)
	}

	// Count total
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	limit := filters.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := filters.Offset
	if offset < 0 {
		offset = 0
	}

	// Order by relevance if search query, otherwise by created_at
	if query != "" {
		// Use raw SQL for ts_rank ordering with the sanitized query
		db = db.Order(clause.Expr{
			SQL:  "ts_rank(search_vector, plainto_tsquery('english', ?)) DESC",
			Vars: []interface{}{sanitizedQuery},
		})
	} else {
		db = db.Order("created_at DESC")
	}

	if err := db.Limit(limit).Offset(offset).Find(&places).Error; err != nil {
		return nil, 0, err
	}

	return places, total, nil
}

// FindNearby finds places within a radius of coordinates using geohash
func (r *placesRepository) FindNearby(ctx context.Context, lat, lng, radiusKm float64, limit int) ([]models.Place, error) {
	var places []models.Place

	if limit <= 0 || limit > 100 {
		limit = 20
	}

	// Use Haversine formula for accurate distance calculation
	// 6371 is Earth's radius in kilometers
	query := `
		SELECT *,
			(6371 * acos(
				cos(radians(?)) * cos(radians(latitude)) *
				cos(radians(longitude) - radians(?)) +
				sin(radians(?)) * sin(radians(latitude))
			)) AS distance
		FROM places
		WHERE deleted_at IS NULL
			AND latitude BETWEEN ? AND ?
			AND longitude BETWEEN ? AND ?
		HAVING distance <= ?
		ORDER BY distance ASC
		LIMIT ?
	`

	// Calculate bounding box for initial filtering (optimization)
	latDelta := radiusKm / 111.0 // 1 degree latitude = ~111 km
	lngDelta := radiusKm / (111.0 * cos(lat*3.14159/180.0))

	minLat := lat - latDelta
	maxLat := lat + latDelta
	minLng := lng - lngDelta
	maxLng := lng + lngDelta

	err := r.db.WithContext(ctx).Raw(query,
		lat, lng, lat,
		minLat, maxLat, minLng, maxLng,
		radiusKm, limit,
	).Scan(&places).Error

	if err != nil {
		return nil, err
	}

	return places, nil
}

// GetByCity retrieves places in a specific city
func (r *placesRepository) GetByCity(ctx context.Context, city, countryCode string, limit, offset int) ([]models.Place, int64, error) {
	var places []models.Place
	var total int64

	db := r.db.WithContext(ctx).Model(&models.Place{}).
		Where("deleted_at IS NULL").
		Where("LOWER(city) = LOWER(?)", city)

	if countryCode != "" {
		db = db.Where("country_code = ?", strings.ToUpper(countryCode))
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	if err := db.Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&places).Error; err != nil {
		return nil, 0, err
	}

	return places, total, nil
}

// GetByPostalCode retrieves places with a specific postal code
func (r *placesRepository) GetByPostalCode(ctx context.Context, postalCode, countryCode string, limit, offset int) ([]models.Place, int64, error) {
	var places []models.Place
	var total int64

	db := r.db.WithContext(ctx).Model(&models.Place{}).
		Where("deleted_at IS NULL").
		Where("postal_code = ?", postalCode)

	if countryCode != "" {
		db = db.Where("country_code = ?", strings.ToUpper(countryCode))
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	if err := db.Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&places).Error; err != nil {
		return nil, 0, err
	}

	return places, total, nil
}

// Update updates an existing place
func (r *placesRepository) Update(ctx context.Context, place *models.Place) error {
	place.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).
		Model(place).
		Where("id = ? AND deleted_at IS NULL", place.ID).
		Updates(place).Error
}

// Delete soft-deletes a place
func (r *placesRepository) Delete(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.Place{}).
		Where("id = ?", id).
		Update("deleted_at", now).Error
}

// BulkUpsert performs batch insert/update of places
func (r *placesRepository) BulkUpsert(ctx context.Context, places []models.Place) error {
	if len(places) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "external_place_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"formatted_address",
				"latitude",
				"longitude",
				"geohash",
				"street_number",
				"street_name",
				"city",
				"district",
				"state_code",
				"state_name",
				"country_code",
				"country_name",
				"postal_code",
				"place_types",
				"confidence",
				"updated_at",
			}),
		}).
		CreateInBatches(places, 100).Error
}

// GetStats returns statistics about stored places
func (r *placesRepository) GetStats(ctx context.Context) (*models.PlacesStats, error) {
	stats := &models.PlacesStats{
		ByCountry:  make(map[string]int64),
		ByProvider: make(map[string]int64),
	}

	// Total places
	if err := r.db.WithContext(ctx).
		Model(&models.Place{}).
		Where("deleted_at IS NULL").
		Count(&stats.TotalPlaces).Error; err != nil {
		return nil, err
	}

	// Verified places
	if err := r.db.WithContext(ctx).
		Model(&models.Place{}).
		Where("deleted_at IS NULL AND verified = true").
		Count(&stats.VerifiedPlaces).Error; err != nil {
		return nil, err
	}

	// By country
	var countryCounts []struct {
		CountryCode string
		Count       int64
	}
	r.db.WithContext(ctx).
		Model(&models.Place{}).
		Select("country_code, COUNT(*) as count").
		Where("deleted_at IS NULL AND country_code IS NOT NULL").
		Group("country_code").
		Order("count DESC").
		Limit(20).
		Scan(&countryCounts)
	for _, cc := range countryCounts {
		stats.ByCountry[cc.CountryCode] = cc.Count
	}

	// By provider
	var providerCounts []struct {
		SourceProvider string
		Count          int64
	}
	r.db.WithContext(ctx).
		Model(&models.Place{}).
		Select("source_provider, COUNT(*) as count").
		Where("deleted_at IS NULL AND source_provider IS NOT NULL").
		Group("source_provider").
		Scan(&providerCounts)
	for _, pc := range providerCounts {
		stats.ByProvider[pc.SourceProvider] = pc.Count
	}

	return stats, nil
}

// SetVerified marks a place as verified
func (r *placesRepository) SetVerified(ctx context.Context, id uuid.UUID, verified bool) error {
	return r.db.WithContext(ctx).
		Model(&models.Place{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"verified":   verified,
			"updated_at": time.Now(),
		}).Error
}

// sanitizeSearchQuery removes special characters from search query
func sanitizeSearchQuery(query string) string {
	// Remove characters that might break tsquery
	replacer := strings.NewReplacer(
		"'", "",
		"\"", "",
		":", "",
		"&", "",
		"|", "",
		"!", "",
		"(", "",
		")", "",
	)
	return replacer.Replace(query)
}

// cos returns the cosine of an angle in radians
func cos(rad float64) float64 {
	// Simple cosine approximation using Taylor series
	// For more accuracy, use math.Cos
	return 1 - (rad*rad)/2 + (rad*rad*rad*rad)/24
}

// PlaceWithDistance is a place with calculated distance
type PlaceWithDistance struct {
	models.Place
	Distance float64 `json:"distance"`
}
