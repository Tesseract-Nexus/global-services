package config

import (
	"os"
	"strconv"

	"github.com/tesseract-hub/go-shared/secrets")

type Config struct {
	Server     ServerConfig
	QR         QRConfig
	Storage    StorageConfig
	Encryption EncryptionConfig
	Database   DatabaseConfig
	LogLevel   string
}

type ServerConfig struct {
	Port         string
	ReadTimeout  int
	WriteTimeout int
}

type QRConfig struct {
	DefaultSize    int
	MaxSize        int
	MinSize        int
	DefaultQuality string
}

type StorageConfig struct {
	Provider   string
	BucketName string
	BasePath   string
	PublicURL  string
	LocalPath  string
}

type EncryptionConfig struct {
	Enabled bool
	Key     string
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

func LoadConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         getEnv("PORT", "8080"),
			ReadTimeout:  getEnvAsInt("READ_TIMEOUT", 30),
			WriteTimeout: getEnvAsInt("WRITE_TIMEOUT", 30),
		},
		QR: QRConfig{
			DefaultSize:    getEnvAsInt("QR_DEFAULT_SIZE", 256),
			MaxSize:        getEnvAsInt("QR_MAX_SIZE", 1024),
			MinSize:        getEnvAsInt("QR_MIN_SIZE", 64),
			DefaultQuality: getEnv("QR_DEFAULT_QUALITY", "medium"),
		},
		Storage: StorageConfig{
			Provider:   getEnv("STORAGE_PROVIDER", "local"),
			BucketName: getEnv("GCS_BUCKET_NAME", ""),
			BasePath:   getEnv("GCS_BASE_PATH", "qr-codes"),
			PublicURL:  getEnv("GCS_PUBLIC_URL", "https://storage.googleapis.com"),
			LocalPath:  getEnv("LOCAL_STORAGE_PATH", "/tmp/qr-codes"),
		},
		Encryption: EncryptionConfig{
			Enabled: getEnvAsBool("ENCRYPTION_ENABLED", true),
			Key:     getEnv("ENCRYPTION_KEY", ""),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: secrets.GetDBPassword(),
			Name:     getEnv("DB_NAME", "tesseract_hub"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		LogLevel: getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}
