package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq" // PostgreSQL driver for sql.DB
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"location-service/internal/config"
	"location-service/internal/events"
	"location-service/internal/handler"
	"location-service/internal/handlers"
	"location-service/internal/middleware"
	"location-service/internal/migration"
	"location-service/internal/models"
	"location-service/internal/repository"
	"location-service/internal/services"
	"location-service/internal/worker"
	"github.com/Tesseract-Nexus/go-shared/metrics"
	"github.com/Tesseract-Nexus/go-shared/rbac"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize database connection (optional for development)
	db, err := initDatabase(cfg)
	if err != nil {
		log.Printf("Warning: Failed to initialize database: %v", err)
		log.Println("Running in mock mode without database...")
		// Continue without database for development/testing
	} else {
		// Auto-migrate models only if database is available
		if err := autoMigrate(db, cfg); err != nil {
			log.Printf("Warning: Failed to migrate database: %v", err)
		} else {
			log.Println("Database initialized successfully")
		}
	}

	// Initialize repositories (with nil handling for mock mode)
	var countryRepo repository.CountryRepository
	var stateRepo repository.StateRepository
	var currencyRepo repository.CurrencyRepository
	var timezoneRepo repository.TimezoneRepository
	var cacheRepo repository.LocationCacheRepository
	var addressCacheRepo repository.AddressCacheRepository
	var placesRepo repository.PlacesRepository

	if db != nil {
		countryRepo = repository.NewCountryRepository(db)
		stateRepo = repository.NewStateRepository(db)
		currencyRepo = repository.NewCurrencyRepository(db)
		timezoneRepo = repository.NewTimezoneRepository(db)
		cacheRepo = repository.NewLocationCacheRepository(db)
		addressCacheRepo = repository.NewAddressCacheRepository(db)
		placesRepo = repository.NewPlacesRepository(db)
	}

	// Initialize services
	locationSvc := services.NewLocationService(countryRepo, stateRepo, currencyRepo, timezoneRepo, cacheRepo)
	geoSvc := services.NewGeoLocationServiceWithProvider(cfg.Services.GeoLocationProvider)

	// Create address service with failover chain: Mapbox → Photon → LocationIQ → OpenStreetMap → Google
	// Google is last (pay-per-use), free providers are prioritized
	addressSvc := services.NewAddressServiceWithFailover(services.AddressServiceConfig{
		MapboxToken:      cfg.Services.MapboxAccessToken,
		GoogleAPIKey:     cfg.Services.GoogleMapsAPIKey,
		HereAPIKey:       cfg.Services.HereAPIKey,
		LocationIQAPIKey: cfg.Services.LocationIQAPIKey,
		PhotonURL:        cfg.Services.PhotonURL,
		EnableFailover:   true,
	})
	log.Printf("✓ Address service initialized with failover (Mapbox → Photon → LocationIQ → OpenStreetMap → Google)")

	// Initialize GeoTag caching and service
	var geotagSvc *services.GeoTagService
	var cachedProvider *services.CachedAddressProvider
	var cleanupWorker *worker.CacheCleanupWorker

	if addressCacheRepo != nil && placesRepo != nil {
		// Configure cache settings from environment
		cacheConfig := services.DefaultCacheConfig()
		if ttlDays := os.Getenv("ADDRESS_CACHE_GEOCODE_TTL_DAYS"); ttlDays != "" {
			if days, err := strconv.Atoi(ttlDays); err == nil {
				cacheConfig.GeocodeTTL = time.Duration(days) * 24 * time.Hour
				cacheConfig.ReverseGeocodeTTL = time.Duration(days) * 24 * time.Hour
			}
		}
		if ttlDays := os.Getenv("ADDRESS_CACHE_AUTOCOMPLETE_TTL_DAYS"); ttlDays != "" {
			if days, err := strconv.Atoi(ttlDays); err == nil {
				cacheConfig.AutocompleteTTL = time.Duration(days) * 24 * time.Hour
			}
		}
		if enabled := os.Getenv("ADDRESS_CACHE_ENABLED"); enabled == "false" {
			cacheConfig.Enabled = false
		}
		if storePlaces := os.Getenv("GEOTAG_STORE_PLACES"); storePlaces == "false" {
			cacheConfig.StorePlaces = false
		}

		// Create cached address provider
		cachedProvider = services.NewCachedAddressProvider(
			addressSvc.GetProvider(),
			addressCacheRepo,
			placesRepo,
			cacheConfig,
		)

		// Create GeoTag service
		geotagSvc = services.NewGeoTagService(
			placesRepo,
			addressCacheRepo,
			addressSvc,
			cachedProvider,
		)

		// Start cache cleanup worker
		cleanupConfig := worker.DefaultCacheCleanupConfig()
		if intervalHours := os.Getenv("ADDRESS_CACHE_CLEANUP_INTERVAL_HOURS"); intervalHours != "" {
			if hours, err := strconv.Atoi(intervalHours); err == nil {
				cleanupConfig.Interval = time.Duration(hours) * time.Hour
			}
		}
		cleanupWorker = worker.NewCacheCleanupWorker(addressCacheRepo, cleanupConfig)
		cleanupWorker.Start()

		log.Printf("✓ GeoTag service initialized with caching (TTL: %v geocode, %v autocomplete)",
			cacheConfig.GeocodeTTL, cacheConfig.AutocompleteTTL)
	} else {
		// Create GeoTag service without caching
		geotagSvc = services.NewGeoTagService(nil, nil, addressSvc, nil)
		log.Println("GeoTag service initialized without caching (database not available)")
	}

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler(db)
	locationHandler := handlers.NewLocationHandler(locationSvc, geoSvc)
	addressHandler := handlers.NewAddressHandler(addressSvc)
	geotagHandler := handler.NewGeoTagHandler(geotagSvc)

	// Initialize NATS events publisher (non-blocking)
	eventLogger := logrus.New()
	eventLogger.SetFormatter(&logrus.JSONFormatter{})
	eventLogger.SetLevel(logrus.InfoLevel)
	go func() {
		if err := events.InitPublisher(eventLogger); err != nil {
			log.Printf("WARNING: Failed to initialize events publisher: %v (events won't be published)", err)
		} else {
			log.Println("✓ NATS events publisher initialized")
		}
	}()

	// Initialize metrics
	metricsCollector := initMetrics(db)

	// Initialize RBAC middleware
	staffServiceURL := os.Getenv("STAFF_SERVICE_URL")
	if staffServiceURL == "" {
		staffServiceURL = "http://staff-service:8080"
	}
	rbacMiddleware := rbac.NewMiddlewareWithURL(staffServiceURL, nil)
	log.Println("✓ RBAC middleware initialized")

	// Setup router
	router := setupRouter(healthHandler, locationHandler, addressHandler, geotagHandler, metricsCollector, rbacMiddleware)

	// Setup server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.Port),
		Handler: router,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting location-service on port %s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop cleanup worker
	if cleanupWorker != nil {
		cleanupWorker.Stop()
	}

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

func setupRouter(
	healthHandler *handlers.HealthHandler,
	locationHandler *handlers.LocationHandler,
	addressHandler *handlers.AddressHandler,
	geotagHandler *handler.GeoTagHandler,
	metricsCollector *metrics.Metrics,
	rbacMiddleware *rbac.Middleware,
) *gin.Engine {
	// Set Gin mode
	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Global middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(metricsCollector.Middleware())
	router.Use(middleware.CORS())

	// Health endpoints
	router.GET("/health", healthHandler.Health)
	router.GET("/ready", healthHandler.Ready)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// ===== PUBLIC ENDPOINTS (no RBAC required) =====
		// These endpoints are used by onboarding and public-facing apps

		// Location detection - public access for onboarding
		v1.GET("/location/detect", locationHandler.DetectLocation)

		// Countries - public access for country/state selection
		countries := v1.Group("/countries")
		{
			countries.GET("", locationHandler.GetCountries)
			countries.GET("/:countryId", locationHandler.GetCountry)
			countries.GET("/:countryId/states", locationHandler.GetStates)
		}

		// States - public access for state selection
		states := v1.Group("/states")
		{
			states.GET("", locationHandler.GetAllStates)
			states.GET("/:stateId", locationHandler.GetState)
		}

		// Currencies - public access for currency selection
		currencies := v1.Group("/currencies")
		{
			currencies.GET("", locationHandler.GetCurrencies)
			currencies.GET("/:currencyCode", locationHandler.GetCurrency)
		}

		// Timezones - public access for timezone selection
		timezones := v1.Group("/timezones")
		{
			timezones.GET("", locationHandler.GetTimezones)
			timezones.GET("/:timezone", locationHandler.GetTimezone)
		}

		// Address lookup endpoints - public access for address autocomplete
		address := v1.Group("/address")
		{
			// Autocomplete - for address search suggestions
			address.GET("/autocomplete", addressHandler.Autocomplete)
			address.POST("/autocomplete", addressHandler.AutocompletePost)

			// Geocoding - convert address to coordinates
			address.GET("/geocode", addressHandler.Geocode)
			address.POST("/geocode", addressHandler.GeocodePost)

			// Reverse geocoding - convert coordinates to address
			address.GET("/reverse-geocode", addressHandler.ReverseGeocode)
			address.POST("/reverse-geocode", addressHandler.ReverseGeocodePost)

			// Place details - get full address details from place ID
			address.GET("/place-details", addressHandler.GetPlaceDetails)

			// Validation - validate and standardize address
			address.GET("/validate", addressHandler.ValidateAddress)
			address.POST("/validate", addressHandler.ValidateAddressPost)

			// Manual address entry - fallback when autocomplete doesn't work
			address.POST("/format-manual", addressHandler.FormatManualAddress)

			// Parse address - extract components from free-form address
			address.POST("/parse", addressHandler.ParseAddress)
		}

		// GeoTag API - cached geocoding and places database
		// Public endpoints for geocoding with caching
		geotagHandler.RegisterRoutes(v1)

		// Admin endpoints for CRUD operations with RBAC
		admin := v1.Group("/admin")
		{
			// Admin - Countries with RBAC
			adminCountries := admin.Group("/countries")
			{
				adminCountries.POST("", rbacMiddleware.RequirePermission(rbac.PermissionLocationsCreate), locationHandler.CreateCountry)
				adminCountries.PUT("/:countryId", rbacMiddleware.RequirePermission(rbac.PermissionLocationsUpdate), locationHandler.UpdateCountry)
				adminCountries.DELETE("/:countryId", rbacMiddleware.RequirePermission(rbac.PermissionLocationsDelete), locationHandler.DeleteCountry)
			}

			// Admin - States with RBAC
			adminStates := admin.Group("/states")
			{
				adminStates.POST("", rbacMiddleware.RequirePermission(rbac.PermissionLocationsCreate), locationHandler.CreateState)
				adminStates.PUT("/:stateId", rbacMiddleware.RequirePermission(rbac.PermissionLocationsUpdate), locationHandler.UpdateState)
				adminStates.DELETE("/:stateId", rbacMiddleware.RequirePermission(rbac.PermissionLocationsDelete), locationHandler.DeleteState)
			}

			// Admin - Currencies with RBAC
			adminCurrencies := admin.Group("/currencies")
			{
				adminCurrencies.POST("", rbacMiddleware.RequirePermission(rbac.PermissionLocationsCreate), locationHandler.CreateCurrency)
				adminCurrencies.PUT("/:currencyCode", rbacMiddleware.RequirePermission(rbac.PermissionLocationsUpdate), locationHandler.UpdateCurrency)
				adminCurrencies.DELETE("/:currencyCode", rbacMiddleware.RequirePermission(rbac.PermissionLocationsDelete), locationHandler.DeleteCurrency)
			}

			// Admin - Timezones with RBAC
			adminTimezones := admin.Group("/timezones")
			{
				adminTimezones.POST("", rbacMiddleware.RequirePermission(rbac.PermissionLocationsCreate), locationHandler.CreateTimezone)
				adminTimezones.PUT("/:timezoneId", rbacMiddleware.RequirePermission(rbac.PermissionLocationsUpdate), locationHandler.UpdateTimezone)
				adminTimezones.DELETE("/:timezoneId", rbacMiddleware.RequirePermission(rbac.PermissionLocationsDelete), locationHandler.DeleteTimezone)
			}

			// Admin - Cache management with RBAC
			adminCache := admin.Group("/cache")
			{
				adminCache.GET("/stats", rbacMiddleware.RequirePermission(rbac.PermissionLocationsRead), locationHandler.GetCacheStats)
				adminCache.POST("/cleanup", rbacMiddleware.RequirePermission(rbac.PermissionLocationsUpdate), locationHandler.CleanupCache)
			}
		}
	}

	return router
}

func initDatabase(cfg *config.Config) (*gorm.DB, error) {
	// Build connection string with schema search_path
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s search_path=%s,public",
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Name,
		cfg.Database.SSLMode,
		cfg.Database.Schema,
	)

	// Connect to database
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Test connection
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("Connected to database successfully")
	return db, nil
}

func autoMigrate(db *gorm.DB, cfg *config.Config) error {
	log.Println("Starting database migration...")

	// First, run SQL migrations using the migration package
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s search_path=%s,public",
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Name,
		cfg.Database.SSLMode,
		cfg.Database.Schema,
	)

	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Printf("Warning: Failed to open SQL connection for migrations: %v", err)
		// Fall back to GORM auto-migrate
	} else {
		defer sqlDB.Close()

		migrator := migration.NewMigrator(sqlDB)
		if err := migrator.RunMigrations(); err != nil {
			log.Printf("Warning: SQL migrations failed: %v, falling back to GORM auto-migrate", err)
		} else {
			log.Println("SQL migrations completed successfully")
			return nil
		}
	}

	// Fallback: Auto-migrate all models using GORM
	log.Println("Running GORM auto-migrate as fallback...")
	if err := db.AutoMigrate(
		&models.Country{},
		&models.State{},
		&models.Currency{},
		&models.Timezone{},
		&models.LocationCache{},
	); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	log.Println("Database migration completed")
	return nil
}

func initMetrics(db *gorm.DB) *metrics.Metrics {
	// Initialize base metrics collector
	m := metrics.New(metrics.Config{
		ServiceName: "location-service",
		Namespace:   "tesseract",
		Subsystem:   "location",
	})

	// Register location-specific business metrics
	locationDetections := promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "tesseract",
			Subsystem: "location",
			Name:      "detections_total",
			Help:      "Total number of location detection requests",
		},
		[]string{"method"}, // ip, headers, etc.
	)

	countryLookups := promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "tesseract",
			Subsystem: "location",
			Name:      "country_lookups_total",
			Help:      "Total number of country lookup requests",
		},
		[]string{"result"}, // success, not_found
	)

	stateLookups := promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "tesseract",
			Subsystem: "location",
			Name:      "state_lookups_total",
			Help:      "Total number of state lookup requests",
		},
		[]string{"result"}, // success, not_found
	)

	cacheHits := promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "tesseract",
			Subsystem: "location",
			Name:      "cache_hits_total",
			Help:      "Total number of cache hits",
		},
	)

	cacheMisses := promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "tesseract",
			Subsystem: "location",
			Name:      "cache_misses_total",
			Help:      "Total number of cache misses",
		},
	)

	// Only initialize database metrics if database is available
	if db != nil {
		// Database connection pool metrics
		dbConnectionsOpen := promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "tesseract",
				Subsystem: "location",
				Name:      "db_connections_open",
				Help:      "Number of open database connections",
			},
		)

		dbConnectionsInUse := promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "tesseract",
				Subsystem: "location",
				Name:      "db_connections_in_use",
				Help:      "Number of database connections currently in use",
			},
		)

		dbConnectionsIdle := promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "tesseract",
				Subsystem: "location",
				Name:      "db_connections_idle",
				Help:      "Number of idle database connections",
			},
		)

		cachedLocations := promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "tesseract",
				Subsystem: "location",
				Name:      "cached_locations",
				Help:      "Number of locations in cache",
			},
		)

		// GeoTag cache metrics
		geotagCacheHits := promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "tesseract",
				Subsystem: "geotag",
				Name:      "cache_hits_total",
				Help:      "Total number of GeoTag cache hits by type",
			},
			[]string{"type"}, // geocode, reverse, autocomplete, place_details
		)

		geotagCacheMisses := promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "tesseract",
				Subsystem: "geotag",
				Name:      "cache_misses_total",
				Help:      "Total number of GeoTag cache misses by type",
			},
			[]string{"type"},
		)

		geotagCacheEntries := promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "tesseract",
				Subsystem: "geotag",
				Name:      "cache_entries",
				Help:      "Current number of entries in GeoTag address cache",
			},
		)

		geotagPlacesTotal := promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "tesseract",
				Subsystem: "geotag",
				Name:      "places_total",
				Help:      "Total number of places stored in the database",
			},
		)

		geotagAPICallsSaved := promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: "tesseract",
				Subsystem: "geotag",
				Name:      "api_calls_saved_total",
				Help:      "Total number of external API calls saved by caching",
			},
		)

		// Start goroutine to update database metrics periodically
		go func() {
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()

			for range ticker.C {
				// Update database connection pool metrics
				sqlDB, err := db.DB()
				if err != nil {
					log.Printf("Failed to get database instance for metrics: %v", err)
					continue
				}

				stats := sqlDB.Stats()
				dbConnectionsOpen.Set(float64(stats.OpenConnections))
				dbConnectionsInUse.Set(float64(stats.InUse))
				dbConnectionsIdle.Set(float64(stats.Idle))

				// Update cached locations count
				var count int64
				db.Model(&models.LocationCache{}).Count(&count)
				cachedLocations.Set(float64(count))

				// Update GeoTag cache entries count
				var cacheCount int64
				db.Table("address_cache").Count(&cacheCount)
				geotagCacheEntries.Set(float64(cacheCount))

				// Update places count
				var placesCount int64
				db.Table("places").Count(&placesCount)
				geotagPlacesTotal.Set(float64(placesCount))
			}
		}()

		// Suppress unused variable warnings
		_ = geotagCacheHits
		_ = geotagCacheMisses
		_ = geotagAPICallsSaved
	}

	// Log metrics initialization
	log.Println("Metrics initialized successfully")
	log.Printf("Registered metrics: detections_total, country_lookups_total, state_lookups_total")
	log.Printf("Cache metrics: cache_hits_total, cache_misses_total")
	if db != nil {
		log.Printf("Database metrics: db_connections_open, db_connections_in_use, db_connections_idle, cached_locations")
		log.Printf("GeoTag metrics: geotag_cache_hits_total, geotag_cache_misses_total, geotag_cache_entries, geotag_places_total, geotag_api_calls_saved_total")
	} else {
		log.Println("Database metrics skipped (running in mock mode)")
	}

	// Suppress unused variable warnings (these will be used by handlers in the future)
	_ = locationDetections
	_ = countryLookups
	_ = stateLookups
	_ = cacheHits
	_ = cacheMisses

	return m
}
