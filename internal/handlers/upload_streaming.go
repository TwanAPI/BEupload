package handlers

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"cloud-caddy-backend/internal/database"
	"cloud-caddy-backend/internal/models"
	"cloud-caddy-backend/internal/utils"
)

// UploadStream handles streaming file upload via PUT request
// Reads directly from request body — NO multipart parsing overhead
// Metadata passed via HTTP headers:
//
//	X-File-Name: original filename
//	X-File-Size: file size in bytes (optional, for pre-allocation)
//	X-Description: file description (optional)
func UploadStream(c *gin.Context) {
	// Concurrency control: reject if server at capacity
	if !acquireUploadSlot() {
		c.JSON(503, models.ErrorResponse{Error: "Server at upload capacity, please retry"})
		return
	}
	defer releaseUploadSlot()

	// Get userID from gin context (set by JWTAuth middleware)
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(401, models.ErrorResponse{Error: "Unauthorized"})
		return
	}

	// Read metadata from headers
	fileName := strings.TrimSpace(c.GetHeader("X-File-Name"))
	fileSizeStr := strings.TrimSpace(c.GetHeader("X-File-Size"))
	description := c.GetHeader("X-Description")

	// Validate X-File-Name is not empty, sanitize it
	if fileName == "" {
		c.JSON(400, models.ErrorResponse{Error: "X-File-Name header is required"})
		return
	}
	fileName = filepath.Base(fileName)
	fileName = utils.SanitizeInput(fileName, 255)
	if fileName == "" || fileName == "." || fileName == "/" || fileName == "\\" {
		c.JSON(400, models.ErrorResponse{Error: "Invalid X-File-Name header"})
		return
	}

	description = utils.SanitizeInput(description, 1000)

	// If X-File-Size provided, validate it's > 0 and <= 10GB
	var fileSize int64
	if fileSizeStr != "" {
		parsedSize, err := strconv.ParseInt(fileSizeStr, 10, 64)
		if err != nil || parsedSize <= 0 || parsedSize > maxFileSize {
			c.JSON(400, models.ErrorResponse{
				Error: fmt.Sprintf("File size must be between 1 byte and %s", utils.FormatFileSize(maxFileSize)),
			})
			return
		}
		fileSize = parsedSize
	}

	// Verify request body is present
	if c.Request.Body == nil {
		c.JSON(400, models.ErrorResponse{Error: "Request body cannot be empty"})
		return
	}

	// Ensure upload dir exists
	uploadsDir := filepath.Join("uploads")
	if err := utils.EnsureUploadDir(uploadsDir); err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Failed to create upload directory"})
		return
	}

	// Generate unique filename
	uniqueName := utils.GenerateUniqueFileName(fileName)
	destPath := filepath.Join(uploadsDir, uniqueName)

	// Create destination file. If fileSize known, pre-allocate with f.Truncate(fileSize)
	dst, err := os.Create(destPath)
	if err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Failed to create file"})
		return
	}

	if fileSize > 0 {
		if err := dst.Truncate(fileSize); err != nil {
			dst.Close()
			os.Remove(destPath)
			c.JSON(500, models.ErrorResponse{Error: "Failed to allocate file"})
			return
		}
		if _, err := dst.Seek(0, io.SeekStart); err != nil {
			dst.Close()
			os.Remove(destPath)
			c.JSON(500, models.ErrorResponse{Error: "Failed to seek file"})
			return
		}
	}

	// Stream directly from c.Request.Body to file using pooled 32MB buffer
	buf := getBuffer()
	defer putBuffer(buf)

	written, copyErr := io.CopyBuffer(dst, c.Request.Body, *buf)
	dst.Close()

	if copyErr != nil {
		os.Remove(destPath)
		c.JSON(500, models.ErrorResponse{Error: "Failed to save file"})
		return
	}

	// Verify written size
	if fileSize > 0 && written != fileSize {
		os.Remove(destPath)
		c.JSON(400, models.ErrorResponse{
			Error: fmt.Sprintf("File size mismatch: expected %d bytes, got %d bytes", fileSize, written),
		})
		return
	}

	if written <= 0 {
		os.Remove(destPath)
		c.JSON(400, models.ErrorResponse{Error: "Uploaded file is empty"})
		return
	}

	if written > maxFileSize {
		os.Remove(destPath)
		c.JSON(400, models.ErrorResponse{
			Error: fmt.Sprintf("File size must be between 1 byte and %s", utils.FormatFileSize(maxFileSize)),
		})
		return
	}

	// Get actual file size via os.Stat()
	fileInfo, err := os.Stat(destPath)
	if err != nil {
		os.Remove(destPath)
		c.JSON(500, models.ErrorResponse{Error: "Failed to stat file"})
		return
	}
	actualSize := fileInfo.Size()

	if fileSize > 0 && actualSize != fileSize {
		os.Remove(destPath)
		c.JSON(400, models.ErrorResponse{
			Error: fmt.Sprintf("File size mismatch: expected %d bytes, got %d bytes", fileSize, actualSize),
		})
		return
	}

	// Save metadata to MongoDB files collection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filesCollection := database.DB.Collection("files")
	fileDoc := models.File{
		UserID:      userID.(string),
		FileName:    fileName,
		FileSize:    actualSize,
		FileType:    c.GetHeader("Content-Type"),
		StoragePath: uniqueName,
		Description: &description,
		IsPublic:    false,
		CreatedAt:   time.Now().UnixMilli(),
		UpdatedAt:   time.Now().UnixMilli(),
	}

	result, err := filesCollection.InsertOne(ctx, fileDoc)
	if err != nil {
		os.Remove(destPath)
		c.JSON(500, models.ErrorResponse{Error: "Failed to save file metadata"})
		return
	}

	c.JSON(200, models.UploadResponse{
		ID:       result.InsertedID.(primitive.ObjectID).Hex(),
		FileName: fileDoc.FileName,
		FileSize: actualSize,
	})
}
