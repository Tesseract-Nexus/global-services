package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"document-service/internal/models"
	"github.com/Tesseract-Nexus/go-shared/secrets"
)

// Config holds the application configuration
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Storage  StorageConfig  `mapstructure:"storage"`
	Cache    CacheConfig    `mapstructure:"cache"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	Security SecurityConfig `mapstructure:"security"`
}

// CacheConfig holds cache configuration
type CacheConfig struct {
	Enabled           bool   `mapstructure:"enabled" default:"true"`
	Host              string `mapstructure:"host" default:"localhost"`
	Port              string `mapstructure:"port" default:"6379"`
	Password          string `mapstructure:"password"`
	DB                int    `mapstructure:"db" default:"0"`
	PoolSize          int    `mapstructure:"pool_size" default:"10"`
	PresignedURLTTL   int    `mapstructure:"presigned_url_ttl" default:"3300"`   // 55 minutes (URL valid for 1h)
	MetadataTTL       int    `mapstructure:"metadata_ttl" default:"300"`         // 5 minutes
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
	Port         string `mapstructure:"port" default:"8080"`
	Host         string `mapstructure:"host" default:"0.0.0.0"`
	ReadTimeout  int    `mapstructure:"read_timeout" default:"10"`
	WriteTimeout int    `mapstructure:"write_timeout" default:"10"`
	IdleTimeout  int    `mapstructure:"idle_timeout" default:"60"`
}

// DatabaseConfig holds database-related configuration
type DatabaseConfig struct {
	Host         string `mapstructure:"host" default:"localhost"`
	Port         string `mapstructure:"port" default:"5432"`
	Name         string `mapstructure:"name" default:"document_service"`
	User         string `mapstructure:"user" default:"postgres"`
	Password     string `mapstructure:"password"`
	SSLMode      string `mapstructure:"ssl_mode" default:"disable"`
	MaxOpenConns int    `mapstructure:"max_open_conns" default:"25"`
	MaxIdleConns int    `mapstructure:"max_idle_conns" default:"10"`
	MaxLifetime  int    `mapstructure:"max_lifetime" default:"300"`
}

// StorageConfig holds cloud storage configuration
type StorageConfig struct {
	Provider         models.CloudProvider `mapstructure:"provider" default:"aws"`
	DefaultBucket    string               `mapstructure:"default_bucket"`
	PublicBucket     string               `mapstructure:"public_bucket"`  // Public bucket for marketplace assets
	PublicBucketURL  string               `mapstructure:"public_bucket_url"` // CDN URL or direct GCS URL for public bucket
	MaxFileSize      int64                `mapstructure:"max_file_size" default:"104857600"` // 100MB
	AllowedMimeTypes []string             `mapstructure:"allowed_mime_types"`

	// Validation settings
	ValidateFileType    bool `mapstructure:"validate_file_type" default:"true"`
	EnableVirusScanning bool `mapstructure:"enable_virus_scanning" default:"false"`
	EnableThumbnails    bool `mapstructure:"enable_thumbnails" default:"false"`

	// Cache settings
	DefaultCacheControl string `mapstructure:"default_cache_control" default:"public, max-age=3600"`

	// Provider-specific configs
	AWS   models.AWSConfig   `mapstructure:"aws"`
	Azure models.AzureConfig `mapstructure:"azure"`
	GCP   models.GCPConfig   `mapstructure:"gcp"`
	Local models.LocalConfig `mapstructure:"local"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level      string `mapstructure:"level" default:"info"`
	Format     string `mapstructure:"format" default:"json"`
	Output     string `mapstructure:"output" default:"stdout"`
	TimeFormat string `mapstructure:"time_format" default:"2006-01-02T15:04:05Z07:00"`
}

// SecurityConfig holds security-related configuration
type SecurityConfig struct {
	EnableCORS      bool     `mapstructure:"enable_cors" default:"true"`
	AllowedOrigins  []string `mapstructure:"allowed_origins"`
	AllowedHeaders  []string `mapstructure:"allowed_headers"`
	AllowedMethods  []string `mapstructure:"allowed_methods"`
	EnableTLS       bool     `mapstructure:"enable_tls" default:"false"`
	TLSCertFile     string   `mapstructure:"tls_cert_file"`
	TLSKeyFile      string   `mapstructure:"tls_key_file"`
	EnableAuth      bool     `mapstructure:"enable_auth" default:"true"`
	JWTSecret       string   `mapstructure:"jwt_secret"`
	EnableRateLimit bool     `mapstructure:"enable_rate_limit" default:"true"`
	RateLimitPerMin int      `mapstructure:"rate_limit_per_min" default:"100"`
}

// LoadConfig loads configuration from various sources
func LoadConfig() (*Config, error) {
	config := &Config{}

	// Set configuration file search paths
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("/etc/document-service")
	viper.AddConfigPath("$HOME/.document-service")

	// Set configuration file name and type
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// Enable environment variable reading
	viper.SetEnvPrefix("DOC_SERVICE")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	// Set default values
	setDefaults()

	// Read configuration file if it exists
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		// Config file not found, which is fine - we'll use defaults and env vars
	}

	// Override with environment variables for common use cases
	bindEnvVars()

	// Unmarshal into config struct
	if err := viper.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// setDefaults sets default configuration values
func setDefaults() {
	// Server defaults
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.read_timeout", 10)
	viper.SetDefault("server.write_timeout", 10)
	viper.SetDefault("server.idle_timeout", 60)

	// Database defaults
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", "5432")
	viper.SetDefault("database.name", "document_service")
	viper.SetDefault("database.user", "postgres")
	viper.SetDefault("database.ssl_mode", "disable")
	viper.SetDefault("database.max_open_conns", 25)
	viper.SetDefault("database.max_idle_conns", 10)
	viper.SetDefault("database.max_lifetime", 300)

	// Storage defaults
	viper.SetDefault("storage.provider", "aws")
	viper.SetDefault("storage.max_file_size", 104857600) // 100MB
	viper.SetDefault("storage.validate_file_type", true)
	viper.SetDefault("storage.enable_virus_scanning", false)
	viper.SetDefault("storage.enable_thumbnails", false)
	viper.SetDefault("storage.default_cache_control", "public, max-age=3600")

	// Cache defaults
	viper.SetDefault("cache.enabled", true)
	viper.SetDefault("cache.host", "localhost")
	viper.SetDefault("cache.port", "6379")
	viper.SetDefault("cache.db", 0)
	viper.SetDefault("cache.pool_size", 10)
	viper.SetDefault("cache.presigned_url_ttl", 3300) // 55 minutes
	viper.SetDefault("cache.metadata_ttl", 300)        // 5 minutes

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("logging.output", "stdout")
	viper.SetDefault("logging.time_format", "2006-01-02T15:04:05Z07:00")

	// Security defaults
	viper.SetDefault("security.enable_cors", true)
	viper.SetDefault("security.enable_tls", false)
	viper.SetDefault("security.enable_auth", true)
	viper.SetDefault("security.enable_rate_limit", true)
	viper.SetDefault("security.rate_limit_per_min", 100)
}

// bindEnvVars binds environment variables to configuration keys
func bindEnvVars() {
	// Database
	viper.BindEnv("database.host", "DB_HOST")
	viper.BindEnv("database.port", "DB_PORT")
	viper.BindEnv("database.name", "DB_NAME")
	viper.BindEnv("database.user", "DB_USER")
	// DB password is loaded from GCP Secret Manager or env var via secrets package
	viper.Set("database.password", secrets.GetDBPassword())
	viper.BindEnv("database.ssl_mode", "DB_SSLMODE")

	// Storage
	viper.BindEnv("storage.provider", "STORAGE_PROVIDER")
	viper.BindEnv("storage.default_bucket", "STORAGE_DEFAULT_BUCKET")
	viper.BindEnv("storage.public_bucket", "STORAGE_PUBLIC_BUCKET")
	viper.BindEnv("storage.public_bucket_url", "STORAGE_PUBLIC_BUCKET_URL")
	viper.BindEnv("storage.max_file_size", "STORAGE_MAX_FILE_SIZE")

	// AWS
	viper.BindEnv("storage.aws.region", "AWS_REGION")
	viper.BindEnv("storage.aws.access_key_id", "AWS_ACCESS_KEY_ID")
	viper.BindEnv("storage.aws.secret_access_key", "AWS_SECRET_ACCESS_KEY")
	viper.BindEnv("storage.aws.session_token", "AWS_SESSION_TOKEN")
	viper.BindEnv("storage.aws.endpoint", "AWS_ENDPOINT")
	viper.BindEnv("storage.aws.force_path_style", "AWS_FORCE_PATH_STYLE")

	// Azure
	viper.BindEnv("storage.azure.account_name", "AZURE_STORAGE_ACCOUNT_NAME")
	viper.BindEnv("storage.azure.account_key", "AZURE_STORAGE_ACCOUNT_KEY")
	viper.BindEnv("storage.azure.sas_token", "AZURE_STORAGE_SAS_TOKEN")
	viper.BindEnv("storage.azure.connection_string", "AZURE_STORAGE_CONNECTION_STRING")

	// GCP
	viper.BindEnv("storage.gcp.project_id", "GOOGLE_CLOUD_PROJECT")
	viper.BindEnv("storage.gcp.key_filename", "GOOGLE_APPLICATION_CREDENTIALS")

	// Local filesystem
	viper.BindEnv("storage.local.base_path", "STORAGE_PATH")

	// Cache (Redis)
	viper.BindEnv("cache.enabled", "CACHE_ENABLED")
	viper.BindEnv("cache.host", "REDIS_HOST")
	viper.BindEnv("cache.port", "REDIS_PORT")
	// Redis password is loaded from GCP Secret Manager or REDIS_PASSWORD env var
	viper.Set("cache.password", secrets.GetRedisPassword())
	viper.BindEnv("cache.db", "REDIS_DB")
	viper.BindEnv("cache.pool_size", "REDIS_POOL_SIZE")
	viper.BindEnv("cache.presigned_url_ttl", "CACHE_PRESIGNED_URL_TTL")
	viper.BindEnv("cache.metadata_ttl", "CACHE_METADATA_TTL")

	// Security
	viper.BindEnv("security.jwt_secret", "JWT_SECRET")
	viper.BindEnv("security.enable_auth", "ENABLE_AUTH")
	viper.BindEnv("security.enable_cors", "ENABLE_CORS")

	// Server
	viper.BindEnv("server.port", "PORT")
	viper.BindEnv("server.host", "HOST")
}

// validateConfig validates the loaded configuration
func validateConfig(config *Config) error {
	// Validate storage provider
	switch config.Storage.Provider {
	case models.ProviderAWS:
		if config.Storage.AWS.Region == "" {
			return fmt.Errorf("AWS region is required when using AWS provider")
		}
	case models.ProviderAzure:
		if config.Storage.Azure.AccountName == "" {
			return fmt.Errorf("Azure account name is required when using Azure provider")
		}
	case models.ProviderGCP:
		if config.Storage.GCP.ProjectID == "" {
			return fmt.Errorf("GCP project ID is required when using GCP provider")
		}
	case models.ProviderLocal:
		if config.Storage.Local.BasePath == "" {
			return fmt.Errorf("storage path is required when using local provider")
		}
		// For local provider, default bucket is optional (will use base path)
		if config.Storage.DefaultBucket == "" {
			config.Storage.DefaultBucket = "default"
		}
	default:
		return fmt.Errorf("unsupported storage provider: %s", config.Storage.Provider)
	}

	// Validate default bucket (only for cloud providers)
	if config.Storage.Provider != models.ProviderLocal && config.Storage.DefaultBucket == "" {
		return fmt.Errorf("default bucket is required")
	}

	// Validate max file size
	if config.Storage.MaxFileSize <= 0 {
		return fmt.Errorf("max file size must be greater than 0")
	}

	// Validate JWT secret if auth is enabled
	if config.Security.EnableAuth && config.Security.JWTSecret == "" {
		return fmt.Errorf("JWT secret is required when authentication is enabled")
	}

	// Validate TLS configuration
	if config.Security.EnableTLS {
		if config.Security.TLSCertFile == "" || config.Security.TLSKeyFile == "" {
			return fmt.Errorf("TLS cert and key files are required when TLS is enabled")
		}

		// Check if files exist
		if _, err := os.Stat(config.Security.TLSCertFile); os.IsNotExist(err) {
			return fmt.Errorf("TLS cert file does not exist: %s", config.Security.TLSCertFile)
		}
		if _, err := os.Stat(config.Security.TLSKeyFile); os.IsNotExist(err) {
			return fmt.Errorf("TLS key file does not exist: %s", config.Security.TLSKeyFile)
		}
	}

	return nil
}

// GetDSN returns the database connection string
func (c *Config) GetDSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Database.Host,
		c.Database.Port,
		c.Database.User,
		c.Database.Password,
		c.Database.Name,
		c.Database.SSLMode,
	)
}

// GetAddr returns the server address
func (c *Config) GetAddr() string {
	return fmt.Sprintf("%s:%s", c.Server.Host, c.Server.Port)
}

// IsProduction returns true if running in production mode
func (c *Config) IsProduction() bool {
	env := os.Getenv("GO_ENV")
	return env == "production" || env == "prod"
}

// IsDevelopment returns true if running in development mode
func (c *Config) IsDevelopment() bool {
	env := os.Getenv("GO_ENV")
	return env == "development" || env == "dev" || env == ""
}

// GetConfigPath returns the configuration file path if it exists
func GetConfigPath() string {
	configPath := viper.ConfigFileUsed()
	if configPath != "" {
		return configPath
	}

	// Try to find config file in standard locations
	paths := []string{
		"./config.yaml",
		"./config/config.yaml",
		"/etc/document-service/config.yaml",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// SaveConfig saves the current configuration to a file
func (c *Config) SaveConfig(path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	viper.SetConfigFile(path)
	return viper.WriteConfig()
}

// ConfigProvider implementation
func (c *Config) GetCloudProvider() models.CloudProvider {
	return c.Storage.Provider
}

func (c *Config) GetDefaultBucket() string {
	return c.Storage.DefaultBucket
}

func (c *Config) GetPublicBucket() string {
	return c.Storage.PublicBucket
}

func (c *Config) GetPublicBucketURL() string {
	return c.Storage.PublicBucketURL
}

func (c *Config) GetMaxFileSize() int64 {
	return c.Storage.MaxFileSize
}

func (c *Config) GetAllowedMimeTypes() []string {
	return c.Storage.AllowedMimeTypes
}

func (c *Config) IsValidationEnabled() bool {
	return c.Storage.ValidateFileType
}

func (c *Config) GetAWSConfig() *models.AWSConfig {
	return &c.Storage.AWS
}

func (c *Config) GetAzureConfig() *models.AzureConfig {
	return &c.Storage.Azure
}

func (c *Config) GetGCPConfig() *models.GCPConfig {
	return &c.Storage.GCP
}

func (c *Config) GetLocalConfig() *models.LocalConfig {
	return &c.Storage.Local
}

func (c *Config) GetCacheConfig() *CacheConfig {
	return &c.Cache
}

func (c *Config) IsCacheEnabled() bool {
	return c.Cache.Enabled
}
