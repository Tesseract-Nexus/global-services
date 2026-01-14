package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"qr-service/internal/config"
	"qr-service/internal/events"
	"qr-service/internal/handlers"
	"qr-service/internal/middleware"
	"qr-service/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func main() {
	cfg := config.LoadConfig()

	setupLogger(cfg.LogLevel)

	logrus.WithFields(logrus.Fields{
		"service": "qr-service",
		"port":    cfg.Server.Port,
	}).Info("Starting QR Service")

	var encryptionService *services.EncryptionService
	if cfg.Encryption.Enabled && cfg.Encryption.Key != "" {
		var err error
		encryptionService, err = services.NewEncryptionService(cfg.Encryption.Key)
		if err != nil {
			logrus.WithError(err).Warn("Failed to initialize encryption service, continuing without encryption")
		} else {
			logrus.Info("Encryption service initialized")
		}
	}

	var storageService services.StorageInterface
	ctx := context.Background()

	if cfg.Storage.Provider == "gcs" && cfg.Storage.BucketName != "" {
		gcsStorage, err := services.NewStorageService(ctx, services.StorageConfig{
			BucketName: cfg.Storage.BucketName,
			BasePath:   cfg.Storage.BasePath,
			PublicURL:  cfg.Storage.PublicURL,
		})
		if err != nil {
			logrus.WithError(err).Warn("Failed to initialize GCS storage, using local storage")
			storageService = services.NewLocalStorageService(cfg.Storage.LocalPath, "http://localhost:"+cfg.Server.Port+"/static")
		} else {
			storageService = gcsStorage
			logrus.Info("GCS storage service initialized")
		}
	} else {
		storageService = services.NewLocalStorageService(cfg.Storage.LocalPath, "http://localhost:"+cfg.Server.Port+"/static")
		logrus.Info("Local storage service initialized")
	}

	qrService := services.NewQRService(&cfg.QR, encryptionService, storageService)

	qrHandlers := handlers.NewQRHandlers(qrService, storageService)
	healthHandlers := handlers.NewHealthHandlers()

	// Initialize NATS events publisher (non-blocking)
	go func() {
		eventLogger := logrus.New()
		eventLogger.SetFormatter(&logrus.JSONFormatter{})
		eventLogger.SetLevel(logrus.InfoLevel)
		if err := events.InitPublisher(eventLogger); err != nil {
			logrus.WithError(err).Warn("Failed to initialize events publisher (events won't be published)")
		} else {
			logrus.Info("NATS events publisher initialized")
		}
	}()

	router := setupRouter(qrHandlers, healthHandlers)

	server := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Fatal("Failed to start server")
		}
	}()

	logrus.WithField("port", cfg.Server.Port).Info("Server started successfully")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logrus.Info("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logrus.WithError(err).Error("Server forced to shutdown")
	}

	if storageService != nil {
		if err := storageService.Close(); err != nil {
			logrus.WithError(err).Error("Failed to close storage service")
		}
	}

	logrus.Info("Server exited properly")
}

func setupLogger(level string) {
	logrus.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339,
	})

	switch level {
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	case "warn":
		logrus.SetLevel(logrus.WarnLevel)
	case "error":
		logrus.SetLevel(logrus.ErrorLevel)
	default:
		logrus.SetLevel(logrus.InfoLevel)
	}
}

func setupRouter(qrHandlers *handlers.QRHandlers, healthHandlers *handlers.HealthHandlers) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	if os.Getenv("GIN_MODE") == "debug" {
		gin.SetMode(gin.DebugMode)
	}

	router := gin.New()

	router.Use(gin.Recovery())
	router.Use(middleware.RequestID())
	router.Use(middleware.CORS())
	router.Use(middleware.Logger())
	router.Use(middleware.ErrorHandler())
	router.Use(middleware.TenantExtractor())

	router.GET("/health", healthHandlers.Health)
	router.GET("/ready", healthHandlers.Ready)

	v1 := router.Group("/api/v1")
	{
		qr := v1.Group("/qr")
		{
			qr.POST("/generate", qrHandlers.GenerateQRCode)

			qr.GET("/image", qrHandlers.GenerateQRCodeImage)

			qr.POST("/download", qrHandlers.DownloadQRCode)

			qr.POST("/batch", qrHandlers.BatchGenerateQRCodes)

			qr.GET("/types", qrHandlers.GetQRTypes)

			qr.POST("/upload", qrHandlers.UploadQRCode)
		}
	}

	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"error":      "Not Found",
			"code":       "NOT_FOUND",
			"request_id": c.GetString("request_id"),
		})
	})

	return router
}

func init() {
	fmt.Println(`
   ____  _____    ____                  _
  / __ \|  __ \  / ___| ___ _ ____   _(_) ___ ___
 | |  | | |__) | \___ \/ _ \ '__\ \ / / |/ __/ _ \
 | |__| |  _  /   ___) |  __/ |   \ V /| | (_|  __/
  \___\_\_| \_\  |____/ \___|_|    \_/ |_|\___\___|

  QR Code Generator Microservice v1.0.0
  `)
}
