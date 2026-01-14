package utils

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// DetectMimeType detects MIME type from filename
func DetectMimeType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))

	mimeTypes := map[string]string{
		// Images
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".webp": "image/webp",
		".svg":  "image/svg+xml",
		".bmp":  "image/bmp",
		".ico":  "image/x-icon",

		// Documents
		".pdf":  "application/pdf",
		".doc":  "application/msword",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".xls":  "application/vnd.ms-excel",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		".ppt":  "application/vnd.ms-powerpoint",
		".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		".txt":  "text/plain",
		".rtf":  "application/rtf",
		".odt":  "application/vnd.oasis.opendocument.text",
		".ods":  "application/vnd.oasis.opendocument.spreadsheet",
		".odp":  "application/vnd.oasis.opendocument.presentation",

		// Audio
		".mp3":  "audio/mpeg",
		".wav":  "audio/wav",
		".ogg":  "audio/ogg",
		".m4a":  "audio/mp4",
		".aac":  "audio/aac",
		".flac": "audio/flac",

		// Video
		".mp4":  "video/mp4",
		".avi":  "video/x-msvideo",
		".mov":  "video/quicktime",
		".wmv":  "video/x-ms-wmv",
		".flv":  "video/x-flv",
		".webm": "video/webm",
		".mkv":  "video/x-matroska",

		// Archives
		".zip": "application/zip",
		".rar": "application/vnd.rar",
		".7z":  "application/x-7z-compressed",
		".tar": "application/x-tar",
		".gz":  "application/gzip",
		".bz2": "application/x-bzip2",

		// Code
		".html": "text/html",
		".htm":  "text/html",
		".css":  "text/css",
		".js":   "application/javascript",
		".json": "application/json",
		".xml":  "application/xml",
		".csv":  "text/csv",
		".yaml": "application/x-yaml",
		".yml":  "application/x-yaml",

		// Others
		".exe": "application/octet-stream",
		".dmg": "application/x-apple-diskimage",
		".iso": "application/x-iso9660-image",
	}

	if mimeType, exists := mimeTypes[ext]; exists {
		return mimeType
	}

	return "application/octet-stream"
}

// SanitizeFilename sanitizes a filename to be safe for storage
func SanitizeFilename(filename string) string {
	// Replace invalid characters with underscores
	reg := regexp.MustCompile(`[<>:"/\\|?*]`)
	sanitized := reg.ReplaceAllString(filename, "_")

	// Remove control characters
	reg = regexp.MustCompile(`[\x00-\x1f\x80-\x9f]`)
	sanitized = reg.ReplaceAllString(sanitized, "")

	// Trim spaces and dots from the ends
	sanitized = strings.TrimSpace(sanitized)
	sanitized = strings.Trim(sanitized, ".")

	// Ensure it's not empty
	if sanitized == "" {
		sanitized = "unnamed_file"
	}

	// Limit length (keeping extension)
	ext := filepath.Ext(sanitized)
	nameWithoutExt := strings.TrimSuffix(sanitized, ext)

	const maxNameLength = 100
	if len(nameWithoutExt) > maxNameLength {
		nameWithoutExt = nameWithoutExt[:maxNameLength]
	}

	return nameWithoutExt + ext
}

// GetFileExtension returns the file extension in lowercase
func GetFileExtension(filename string) string {
	return strings.ToLower(filepath.Ext(filename))
}

// IsImageFile checks if the file is an image based on extension
func IsImageFile(filename string) bool {
	ext := GetFileExtension(filename)
	imageExts := []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".bmp", ".ico"}

	for _, imgExt := range imageExts {
		if ext == imgExt {
			return true
		}
	}
	return false
}

// IsVideoFile checks if the file is a video based on extension
func IsVideoFile(filename string) bool {
	ext := GetFileExtension(filename)
	videoExts := []string{".mp4", ".avi", ".mov", ".wmv", ".flv", ".webm", ".mkv", ".m4v"}

	for _, vidExt := range videoExts {
		if ext == vidExt {
			return true
		}
	}
	return false
}

// IsAudioFile checks if the file is an audio file based on extension
func IsAudioFile(filename string) bool {
	ext := GetFileExtension(filename)
	audioExts := []string{".mp3", ".wav", ".ogg", ".m4a", ".aac", ".flac", ".wma"}

	for _, audExt := range audioExts {
		if ext == audExt {
			return true
		}
	}
	return false
}

// IsDocumentFile checks if the file is a document based on extension
func IsDocumentFile(filename string) bool {
	ext := GetFileExtension(filename)
	docExts := []string{".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".txt", ".rtf", ".odt", ".ods", ".odp"}

	for _, docExt := range docExts {
		if ext == docExt {
			return true
		}
	}
	return false
}

// IsArchiveFile checks if the file is an archive based on extension
func IsArchiveFile(filename string) bool {
	ext := GetFileExtension(filename)
	archiveExts := []string{".zip", ".rar", ".7z", ".tar", ".gz", ".bz2"}

	for _, archExt := range archiveExts {
		if ext == archExt {
			return true
		}
	}
	return false
}

// FormatFileSize formats file size in human-readable format
func FormatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), units[exp])
}

// GenerateUniqueFilename generates a unique filename by appending a number if needed
func GenerateUniqueFilename(filename string, existingFiles []string) string {
	if !contains(existingFiles, filename) {
		return filename
	}

	ext := filepath.Ext(filename)
	nameWithoutExt := strings.TrimSuffix(filename, ext)

	counter := 1
	for {
		newFilename := fmt.Sprintf("%s_%d%s", nameWithoutExt, counter, ext)
		if !contains(existingFiles, newFilename) {
			return newFilename
		}
		counter++
	}
}

// ValidateFilename validates if a filename is acceptable
func ValidateFilename(filename string) error {
	if filename == "" {
		return fmt.Errorf("filename cannot be empty")
	}

	if len(filename) > 255 {
		return fmt.Errorf("filename too long (max 255 characters)")
	}

	// Check for invalid characters
	invalidChars := `<>:"/\|?*`
	for _, char := range invalidChars {
		if strings.ContainsRune(filename, char) {
			return fmt.Errorf("filename contains invalid character: %c", char)
		}
	}

	// Check for reserved names (Windows)
	reservedNames := []string{
		"CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9",
	}

	nameWithoutExt := strings.TrimSuffix(filename, filepath.Ext(filename))
	for _, reserved := range reservedNames {
		if strings.EqualFold(nameWithoutExt, reserved) {
			return fmt.Errorf("filename is a reserved name: %s", reserved)
		}
	}

	return nil
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
