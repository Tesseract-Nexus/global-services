package config

import (
	"log"
	"os"

	"github.com/tesseract-hub/go-shared/secrets")

// Config holds the configuration for the location service
type Config struct {
	Port     string
	Database DatabaseConfig
	Services ServicesConfig
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
	Schema   string
}

// ServicesConfig holds external service configurations
type ServicesConfig struct {
	GeoLocationProvider string // "mock", "maxmind", "ipapi"
	MaxMindLicenseKey   string
	IPAPIKey            string
	// Address lookup services with failover support
	// Failover chain: Mapbox → Photon → LocationIQ → OpenStreetMap → Google
	AddressProvider   string // "mock", "google", "mapbox", "here", "locationiq", "photon", "openstreetmap", "failover"
	GoogleMapsAPIKey  string
	MapboxAccessToken string
	HereAPIKey        string
	LocationIQAPIKey  string // Free tier: 5k requests/day
	PhotonURL         string // Custom Photon URL (default: https://photon.komoot.io)
}

// Load loads configuration from environment variables
func Load() *Config {
	return &Config{
		Port: getEnv("PORT", "8087"),
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: secrets.GetDBPassword(),
			Name:     getEnv("DB_NAME", "tesseract_hub"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
			Schema:   getEnv("DB_SCHEMA", "location"),
		},
		Services: ServicesConfig{
			GeoLocationProvider: getEnv("GEO_PROVIDER", "mock"),
			MaxMindLicenseKey:   getEnv("MAXMIND_LICENSE_KEY", ""),
			IPAPIKey:            getEnv("IPAPI_KEY", ""),
			AddressProvider:     getEnv("ADDRESS_PROVIDER", "failover"),
			GoogleMapsAPIKey:    secrets.GetSecretOrEnv("GOOGLE_MAPS_API_KEY_SECRET_NAME", "GOOGLE_MAPS_API_KEY", ""),
			MapboxAccessToken:   secrets.GetSecretOrEnv("MAPBOX_ACCESS_TOKEN_SECRET_NAME", "MAPBOX_ACCESS_TOKEN", ""),
			HereAPIKey:          secrets.GetSecretOrEnv("HERE_API_KEY_SECRET_NAME", "HERE_API_KEY", ""),
			LocationIQAPIKey:    secrets.GetSecretOrEnv("LOCATIONIQ_API_KEY_SECRET_NAME", "LOCATIONIQ_API_KEY", ""),
			PhotonURL:           getEnv("PHOTON_URL", "https://photon.komoot.io"), // Default to Komoot's public instance
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	log.Printf("Using default value for %s: %s", key, defaultValue)
	return defaultValue
}
