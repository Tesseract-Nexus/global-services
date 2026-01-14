package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"

	"github.com/Tesseract-Nexus/go-shared/secrets")

type Config struct {
	Server      ServerConfig
	Database    DatabaseConfig
	Redis       RedisConfig
	App         AppConfig
	Translation TranslationConfig
}

type ServerConfig struct {
	Host string
	Port int
}

type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

type AppConfig struct {
	Name        string
	Environment string
	LogLevel    string
}

type TranslationConfig struct {
	// LibreTranslate configuration (primary provider - open source)
	LibreTranslateURL string
	LibreTranslateKey string

	// Bergamot configuration (first fallback - Mozilla's fast translation engine)
	BergamotURL string

	// Hugging Face configuration (second fallback - Helsinki-NLP models)
	HuggingFaceURL string
	HuggingFaceKey string

	// Google Cloud Translation configuration (final fallback - paid, most languages)
	GoogleTranslateKey string

	// Cache settings
	CacheTTL         time.Duration
	CacheEnabled     bool

	// Rate limiting
	RateLimit        int
	RateLimitWindow  time.Duration

	// Batch settings
	MaxBatchSize     int
	BatchTimeout     time.Duration

	// Supported languages
	DefaultSourceLang string
	DefaultTargetLang string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	return &Config{
		Server: ServerConfig{
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
			Port: getEnvAsInt("SERVER_PORT", 8080),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvAsInt("DB_PORT", 5432),
			User:     getEnv("DB_USER", "postgres"),
			Password: secrets.GetDBPassword(),
			DBName:   getEnv("DB_NAME", "translation_db"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnvAsInt("REDIS_PORT", 6379),
			Password: secrets.GetRedisPassword(),
			DB:       getEnvAsInt("REDIS_DB", 0),
		},
		App: AppConfig{
			Name:        getEnv("APP_NAME", "translation-service"),
			Environment: getEnv("APP_ENV", "development"),
			LogLevel:    getEnv("LOG_LEVEL", "info"),
		},
		Translation: TranslationConfig{
			LibreTranslateURL:  getEnv("LIBRETRANSLATE_URL", "http://libretranslate:5000"),
			LibreTranslateKey:  getEnv("LIBRETRANSLATE_API_KEY", ""),
			BergamotURL:        getEnv("BERGAMOT_URL", "http://bergamot-service:8080"),
			HuggingFaceURL:     getEnv("HUGGINGFACE_URL", "http://huggingface-mt-service:8080"),
			HuggingFaceKey:     getEnv("HUGGINGFACE_API_KEY", ""),
			GoogleTranslateKey: secrets.GetSecretOrEnv("GOOGLE_TRANSLATE_API_KEY_SECRET_NAME", "GOOGLE_TRANSLATE_API_KEY", ""),
			CacheTTL:          getEnvAsDuration("CACHE_TTL", 24*time.Hour),
			CacheEnabled:      getEnvAsBool("CACHE_ENABLED", true),
			RateLimit:         getEnvAsInt("RATE_LIMIT", 100),
			RateLimitWindow:   getEnvAsDuration("RATE_LIMIT_WINDOW", time.Minute),
			MaxBatchSize:      getEnvAsInt("MAX_BATCH_SIZE", 50),
			BatchTimeout:      getEnvAsDuration("BATCH_TIMEOUT", 30*time.Second),
			DefaultSourceLang: getEnv("DEFAULT_SOURCE_LANG", "en"),
			DefaultTargetLang: getEnv("DEFAULT_TARGET_LANG", "hi"),
		},
	}, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
