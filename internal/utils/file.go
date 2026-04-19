package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// FormatFileSize converts bytes to human-readable format
func FormatFileSize(bytes int64) string {
	units := []string{"B", "KB", "MB", "GB"}
	size := float64(bytes)
	unitIndex := 0

	for size >= 1024 && unitIndex < len(units)-1 {
		size /= 1024
		unitIndex++
	}

	if unitIndex == 0 {
		return fmt.Sprintf("%d %s", int64(size), units[unitIndex])
	}
	return fmt.Sprintf("%.2f %s", size, units[unitIndex])
}

// FormatDate formats time to Vietnamese locale with time
func FormatDate(t time.Time) string {
	return t.Format("02/01/2006 15:04:05")
}

// SanitizeInput sanitizes string input
func SanitizeInput(input string, maxLength int) string {
	if len(input) > maxLength {
		return input[:maxLength]
	}
	return input
}

// EnsureUploadDir ensures upload directory exists
func EnsureUploadDir(uploadDir string) error {
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		return os.MkdirAll(uploadDir, 0755)
	}
	return nil
}

// GenerateUniqueFileName generates a unique filename
func GenerateUniqueFileName(originalName string) string {
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), originalName)
}

// DeleteFile deletes a file from disk
func DeleteFile(filepath string) error {
	return os.Remove(filepath)
}

// GetFileSize gets the size of a file
func GetFileSize(filepath string) (int64, error) {
	info, err := os.Stat(filepath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// PathExists checks if a path exists
func PathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// CleanupOldTempChunks removes temporary chunk files older than specified duration
func CleanupOldTempChunks(tempDir string, duration time.Duration) error {
	files, err := os.ReadDir(tempDir)
	if err != nil {
		return err
	}

	now := time.Now()
	for _, file := range files {
		if !file.IsDir() {
			info, _ := file.Info()
			if now.Sub(info.ModTime()) > duration {
				filePath := filepath.Join(tempDir, file.Name())
				os.Remove(filePath)
			}
		}
	}

	return nil
}
