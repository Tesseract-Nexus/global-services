package utils

import (
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

// SanitizedLogger wraps logrus with automatic PII masking
type SanitizedLogger struct {
	*logrus.Logger
	masker *PIIMasker
}

// SanitizedEntry wraps logrus.Entry with automatic PII masking
type SanitizedEntry struct {
	*logrus.Entry
	masker *PIIMasker
}

// NewSanitizedLogger creates a new logger with PII masking enabled
func NewSanitizedLogger() *SanitizedLogger {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "timestamp",
			logrus.FieldKeyLevel: "level",
			logrus.FieldKeyMsg:   "message",
		},
	})
	logger.SetOutput(os.Stdout)

	// Set log level based on environment
	if os.Getenv("GIN_MODE") == "release" || os.Getenv("GO_ENV") == "production" {
		logger.SetLevel(logrus.InfoLevel)
	} else {
		logger.SetLevel(logrus.DebugLevel)
	}

	return &SanitizedLogger{
		Logger: logger,
		masker: NewPIIMasker(),
	}
}

// SetOutput sets the logger output
func (l *SanitizedLogger) SetOutput(output io.Writer) {
	l.Logger.SetOutput(output)
}

// SetLevel sets the logger level
func (l *SanitizedLogger) SetLevel(level logrus.Level) {
	l.Logger.SetLevel(level)
}

// WithField creates a sanitized entry with a single field
func (l *SanitizedLogger) WithField(key string, value interface{}) *SanitizedEntry {
	sanitizedValue := l.sanitizeValue(key, value)
	return &SanitizedEntry{
		Entry:  l.Logger.WithField(key, sanitizedValue),
		masker: l.masker,
	}
}

// WithFields creates a sanitized entry with multiple fields
func (l *SanitizedLogger) WithFields(fields logrus.Fields) *SanitizedEntry {
	sanitizedFields := l.sanitizeFields(fields)
	return &SanitizedEntry{
		Entry:  l.Logger.WithFields(sanitizedFields),
		masker: l.masker,
	}
}

// WithError creates a sanitized entry with an error
func (l *SanitizedLogger) WithError(err error) *SanitizedEntry {
	sanitizedErr := l.sanitizeError(err)
	return &SanitizedEntry{
		Entry:  l.Logger.WithError(sanitizedErr),
		masker: l.masker,
	}
}

// Info logs at info level with sanitization
func (l *SanitizedLogger) Info(args ...interface{}) {
	l.Logger.Info(l.sanitizeArgs(args)...)
}

// Warn logs at warn level with sanitization
func (l *SanitizedLogger) Warn(args ...interface{}) {
	l.Logger.Warn(l.sanitizeArgs(args)...)
}

// Error logs at error level with sanitization
func (l *SanitizedLogger) Error(args ...interface{}) {
	l.Logger.Error(l.sanitizeArgs(args)...)
}

// Debug logs at debug level with sanitization
func (l *SanitizedLogger) Debug(args ...interface{}) {
	l.Logger.Debug(l.sanitizeArgs(args)...)
}

// Infof logs formatted at info level with sanitization
func (l *SanitizedLogger) Infof(format string, args ...interface{}) {
	l.Logger.Infof(l.masker.MaskAll(format), l.sanitizeArgs(args)...)
}

// Warnf logs formatted at warn level with sanitization
func (l *SanitizedLogger) Warnf(format string, args ...interface{}) {
	l.Logger.Warnf(l.masker.MaskAll(format), l.sanitizeArgs(args)...)
}

// Errorf logs formatted at error level with sanitization
func (l *SanitizedLogger) Errorf(format string, args ...interface{}) {
	l.Logger.Errorf(l.masker.MaskAll(format), l.sanitizeArgs(args)...)
}

// Debugf logs formatted at debug level with sanitization
func (l *SanitizedLogger) Debugf(format string, args ...interface{}) {
	l.Logger.Debugf(l.masker.MaskAll(format), l.sanitizeArgs(args)...)
}

// sanitizeValue sanitizes a single value based on its key
func (l *SanitizedLogger) sanitizeValue(key string, value interface{}) interface{} {
	strVal, ok := value.(string)
	if !ok {
		return value
	}

	// Sensitive field detection
	sensitiveKeys := map[string]func(string) string{
		"ip":           l.masker.MaskIP,
		"ip_address":   l.masker.MaskIP,
		"ipAddress":    l.masker.MaskIP,
		"client_ip":    l.masker.MaskIP,
		"clientIp":     l.masker.MaskIP,
		"remote_addr":  l.masker.MaskIP,
		"remoteAddr":   l.masker.MaskIP,
		"x_forwarded":  l.masker.MaskIP,
		"email":        l.masker.MaskEmail,
		"user_email":   l.masker.MaskEmail,
		"userEmail":    l.masker.MaskEmail,
		"phone":        l.masker.MaskPhone,
		"phone_number": l.masker.MaskPhone,
		"phoneNumber":  l.masker.MaskPhone,
		"mobile":       l.masker.MaskPhone,
		"address":      l.masker.MaskAddress,
		"street":       l.masker.MaskAddress,
		"location":     l.masker.MaskAddress,
	}

	if maskFunc, exists := sensitiveKeys[key]; exists {
		return maskFunc(strVal)
	}

	return value
}

// sanitizeFields sanitizes all fields
func (l *SanitizedLogger) sanitizeFields(fields logrus.Fields) logrus.Fields {
	sanitized := make(logrus.Fields, len(fields))
	for key, value := range fields {
		sanitized[key] = l.sanitizeValue(key, value)
	}
	return sanitized
}

// sanitizeArgs sanitizes all arguments
func (l *SanitizedLogger) sanitizeArgs(args []interface{}) []interface{} {
	sanitized := make([]interface{}, len(args))
	for i, arg := range args {
		if strArg, ok := arg.(string); ok {
			sanitized[i] = l.masker.MaskAll(strArg)
		} else {
			sanitized[i] = arg
		}
	}
	return sanitized
}

// sanitizeError sanitizes error messages
func (l *SanitizedLogger) sanitizeError(err error) error {
	if err == nil {
		return nil
	}
	return &sanitizedError{
		original: err,
		masker:   l.masker,
	}
}

// sanitizedError wraps an error with PII masking
type sanitizedError struct {
	original error
	masker   *PIIMasker
}

func (e *sanitizedError) Error() string {
	return e.masker.MaskAll(e.original.Error())
}

// SanitizedEntry methods

// WithField adds a sanitized field to the entry
func (e *SanitizedEntry) WithField(key string, value interface{}) *SanitizedEntry {
	sanitizedValue := e.sanitizeValue(key, value)
	return &SanitizedEntry{
		Entry:  e.Entry.WithField(key, sanitizedValue),
		masker: e.masker,
	}
}

// WithFields adds sanitized fields to the entry
func (e *SanitizedEntry) WithFields(fields logrus.Fields) *SanitizedEntry {
	sanitizedFields := e.sanitizeFields(fields)
	return &SanitizedEntry{
		Entry:  e.Entry.WithFields(sanitizedFields),
		masker: e.masker,
	}
}

// sanitizeValue sanitizes a single value based on its key
func (e *SanitizedEntry) sanitizeValue(key string, value interface{}) interface{} {
	strVal, ok := value.(string)
	if !ok {
		return value
	}

	sensitiveKeys := map[string]func(string) string{
		"ip":           e.masker.MaskIP,
		"ip_address":   e.masker.MaskIP,
		"ipAddress":    e.masker.MaskIP,
		"client_ip":    e.masker.MaskIP,
		"clientIp":     e.masker.MaskIP,
		"remote_addr":  e.masker.MaskIP,
		"remoteAddr":   e.masker.MaskIP,
		"email":        e.masker.MaskEmail,
		"user_email":   e.masker.MaskEmail,
		"userEmail":    e.masker.MaskEmail,
		"phone":        e.masker.MaskPhone,
		"phone_number": e.masker.MaskPhone,
		"phoneNumber":  e.masker.MaskPhone,
		"address":      e.masker.MaskAddress,
		"street":       e.masker.MaskAddress,
	}

	if maskFunc, exists := sensitiveKeys[key]; exists {
		return maskFunc(strVal)
	}

	return value
}

// sanitizeFields sanitizes all fields
func (e *SanitizedEntry) sanitizeFields(fields logrus.Fields) logrus.Fields {
	sanitized := make(logrus.Fields, len(fields))
	for key, value := range fields {
		sanitized[key] = e.sanitizeValue(key, value)
	}
	return sanitized
}

// Global sanitized logger instance
var Log = NewSanitizedLogger()
