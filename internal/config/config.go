package config

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"
)

// Chunk size thresholds for adaptive buffer sizing.
const (
	Threshold1MB  int64 = 1 * 1024 * 1024  // 1MB
	Threshold10MB int64 = 10 * 1024 * 1024 // 10MB
)

// Default configuration values.
const (
	DefaultMaxConcurrentUploads       = 200
	DefaultUploadBufferSize           = 32 * 1024 * 1024        // 32MB
	DefaultMaxMultipartMemory   int64 = 32 * 1024 * 1024        // 32MB
	DefaultReadHeaderTimeout          = 10 * time.Second
	DefaultIdleTimeout                = 120 * time.Second
	DefaultMaxFileSize          int64 = 10 * 1024 * 1024 * 1024 // 10GB
	DefaultUploadDir                  = "uploads"
	DefaultSmallBufferSize            = 256 * 1024              // 256KB
	DefaultMediumBufferSize           = 4 * 1024 * 1024         // 4MB
	DefaultLargeBufferSize            = 32 * 1024 * 1024        // 32MB
)

// Config defines the performance and system configuration settings.
type Config struct {
	MaxConcurrentUploads int           // MAX_CONCURRENT_UPLOADS (default: 200)
	UploadBufferSize     int           // UPLOAD_BUFFER_SIZE (default: 32MB)
	MaxMultipartMemory   int64         // MAX_MULTIPART_MEMORY (default: 32MB)
	ReadHeaderTimeout    time.Duration // HTTP_READ_HEADER_TIMEOUT (default: 10s)
	IdleTimeout          time.Duration // HTTP_IDLE_TIMEOUT (default: 120s)
	MaxFileSize          int64         // MAX_FILE_SIZE (default: 10GB)
	UploadDir            string        // UPLOAD_DIR (default: "uploads")
	SmallBufferSize      int           // SMALL_BUFFER_SIZE (default: 256KB) — for chunks < 1MB
	MediumBufferSize     int           // MEDIUM_BUFFER_SIZE (default: 4MB) — for chunks 1-10MB
	LargeBufferSize      int           // LARGE_BUFFER_SIZE (default: 32MB) — for chunks > 10MB
}

var (
	cfg  *Config
	lock sync.RWMutex
)

// Load reads configuration from environment variables with sensible defaults,
// stores the result in a package-level variable, and logs the configuration.
func Load() *Config {
	lock.Lock()
	defer lock.Unlock()

	c := &Config{
		MaxConcurrentUploads: getEnvInt("MAX_CONCURRENT_UPLOADS", DefaultMaxConcurrentUploads),
		UploadBufferSize:     getEnvInt("UPLOAD_BUFFER_SIZE", DefaultUploadBufferSize),
		MaxMultipartMemory:   getEnvInt64("MAX_MULTIPART_MEMORY", DefaultMaxMultipartMemory),
		ReadHeaderTimeout:    getEnvDuration("HTTP_READ_HEADER_TIMEOUT", DefaultReadHeaderTimeout),
		IdleTimeout:          getEnvDuration("HTTP_IDLE_TIMEOUT", DefaultIdleTimeout),
		MaxFileSize:          getEnvInt64("MAX_FILE_SIZE", DefaultMaxFileSize),
		UploadDir:            getEnvString("UPLOAD_DIR", DefaultUploadDir),
		SmallBufferSize:      getEnvInt("SMALL_BUFFER_SIZE", DefaultSmallBufferSize),
		MediumBufferSize:     getEnvInt("MEDIUM_BUFFER_SIZE", DefaultMediumBufferSize),
		LargeBufferSize:      getEnvInt("LARGE_BUFFER_SIZE", DefaultLargeBufferSize),
	}

	cfg = c

	fmt.Printf("⚡ Config loaded: MaxConcurrentUploads=%d | UploadBufferSize=%d | MaxMultipartMemory=%d | ReadHeaderTimeout=%v | IdleTimeout=%v | MaxFileSize=%d | UploadDir=%s | SmallBufferSize=%d | MediumBufferSize=%d | LargeBufferSize=%d\n",
		c.MaxConcurrentUploads,
		c.UploadBufferSize,
		c.MaxMultipartMemory,
		c.ReadHeaderTimeout,
		c.IdleTimeout,
		c.MaxFileSize,
		c.UploadDir,
		c.SmallBufferSize,
		c.MediumBufferSize,
		c.LargeBufferSize,
	)

	return cfg
}

// Get returns the loaded configuration, initializing it if not loaded yet.
func Get() *Config {
	lock.RLock()
	if cfg != nil {
		defer lock.RUnlock()
		return cfg
	}
	lock.RUnlock()

	return Load()
}

// GetAdaptiveBufferSize returns the appropriate buffer size based on chunk size thresholds:
//   - chunks < 1MB: SmallBufferSize (256KB)
//   - chunks 1-10MB: MediumBufferSize (4MB)
//   - chunks > 10MB: LargeBufferSize (32MB)
func GetAdaptiveBufferSize(chunkSize int64) int {
	return Get().GetAdaptiveBufferSize(chunkSize)
}

// GetAdaptiveBufferSize returns the appropriate buffer size for a specific Config instance.
func (c *Config) GetAdaptiveBufferSize(chunkSize int64) int {
	if chunkSize < Threshold1MB {
		return c.SmallBufferSize
	}
	if chunkSize <= Threshold10MB {
		return c.MediumBufferSize
	}
	return c.LargeBufferSize
}

// Helper functions for reading and parsing environment variables

func getEnvString(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	parsed, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return parsed
}

func getEnvInt64(key string, defaultVal int64) int64 {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	parsed, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return defaultVal
	}
	return parsed
}

func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	// Try parsing standard duration strings like "10s", "2m"
	if d, err := time.ParseDuration(val); err == nil {
		return d
	}
	// Fallback to integer seconds if numeric value is provided
	if seconds, err := strconv.Atoi(val); err == nil {
		return time.Duration(seconds) * time.Second
	}
	return defaultVal
}
