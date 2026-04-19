package handlers

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"cloud-caddy-backend/internal/database"
	"cloud-caddy-backend/internal/models"
	"cloud-caddy-backend/internal/utils"
)

var (
	uploadSessions sync.Map // map[string]*models.UploadSession

	// ⚡ Buffer pool: reuse 32MB buffers instead of allocating new ones each time
	// Reduces GC pressure by ~90% during high-throughput uploads
	bufferPool = sync.Pool{
		New: func() interface{} {
			buf := make([]byte, 32*1024*1024) // 32MB buffer
			return &buf
		},
	}
)

// getBuffer gets a reusable buffer from the pool
func getBuffer() *[]byte {
	return bufferPool.Get().(*[]byte)
}

// putBuffer returns a buffer to the pool
func putBuffer(buf *[]byte) {
	bufferPool.Put(buf)
}

// UploadChunk handles chunked file upload with sparse file optimization
func UploadChunk(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(401, models.ErrorResponse{Error: "Unauthorized"})
		return
	}

	file, err := c.FormFile("chunk")
	if err != nil {
		c.JSON(400, models.ErrorResponse{Error: "No chunk file provided"})
		return
	}

	uploadID := c.PostForm("uploadId")
	chunkIndexStr := c.PostForm("chunkIndex")
	totalChunksStr := c.PostForm("totalChunks")
	fileName := c.PostForm("fileName")
	description := c.PostForm("description")
	fileSizeStr := c.PostForm("fileSize")
	mimeType := file.Header.Get("Content-Type")
	_ = c.PostForm("isLastChunk") // acknowledged but we detect completion via count

	// Validate metadata
	chunkIndex, err := strconv.Atoi(chunkIndexStr)
	if err != nil {
		c.JSON(400, models.ErrorResponse{Error: "Invalid chunk index"})
		return
	}

	totalChunks, err := strconv.Atoi(totalChunksStr)
	if err != nil {
		c.JSON(400, models.ErrorResponse{Error: "Invalid total chunks"})
		return
	}

	fileSize, err := strconv.ParseInt(fileSizeStr, 10, 64)
	if err != nil {
		c.JSON(400, models.ErrorResponse{Error: "Invalid file size"})
		return
	}

	// Sanitize inputs
	uploadID = utils.SanitizeInput(uploadID, 100)
	fileName = utils.SanitizeInput(fileName, 255)
	description = utils.SanitizeInput(description, 1000)

	// Validate ranges
	if chunkIndex < 0 || chunkIndex >= totalChunks {
		c.JSON(400, models.ErrorResponse{Error: "Invalid chunk index"})
		return
	}

	if totalChunks < 1 || totalChunks > 10000 {
		c.JSON(400, models.ErrorResponse{Error: "Invalid total chunks"})
		return
	}

	// Calculate chunk size for sparse file positioning
	chunkSize := fileSize / int64(totalChunks)
	if fileSize%int64(totalChunks) != 0 {
		chunkSize++ // round up
	}

	// Ensure upload directory exists
	uploadsDir := filepath.Join("uploads")
	utils.EnsureUploadDir(uploadsDir)

	// Get or create upload session
	sessionInterface, loaded := uploadSessions.LoadOrStore(uploadID, &models.UploadSession{
		UploadID:       uploadID,
		UserID:         userID.(string),
		FileName:       fileName,
		Description:    description,
		MimeType:       mimeType,
		FileSize:       fileSize,
		ChunkSize:      chunkSize,
		TotalChunks:    totalChunks,
		ReceivedChunks: make(map[int]bool),
		CreatedAt:      time.Now(),
	})

	session := sessionInterface.(*models.UploadSession)

	// Verify upload belongs to current user
	if session.UserID != userID {
		c.JSON(403, models.ErrorResponse{Error: "Upload does not belong to this user"})
		return
	}

	// ⚡ SPARSE FILE APPROACH: Write chunk directly to final file at correct offset
	// This eliminates the merge step entirely - when all chunks arrive, file is complete!
	if !loaded || session.FinalPath == "" {
		session.Mu.Lock()
		if session.FinalPath == "" {
			finalName := utils.GenerateUniqueFileName(session.FileName)
			session.FinalPath = filepath.Join(uploadsDir, finalName)

			// Pre-allocate file with correct size
			f, err := os.Create(session.FinalPath)
			if err != nil {
				session.Mu.Unlock()
				c.JSON(500, models.ErrorResponse{Error: "Failed to create file"})
				return
			}
			// Truncate to final size (sparse file - only uses disk for written regions)
			if err := f.Truncate(fileSize); err != nil {
				f.Close()
				os.Remove(session.FinalPath)
				session.Mu.Unlock()
				c.JSON(500, models.ErrorResponse{Error: "Failed to allocate file"})
				return
			}
			f.Close()
		}
		session.Mu.Unlock()
	}

	// Open multipart file for reading
	src, err := file.Open()
	if err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Failed to read chunk"})
		return
	}
	defer src.Close()

	// ⚡ Write chunk directly to correct position in final file
	// Calculate byte offset: chunk 0 → offset 0, chunk 1 → offset chunkSize, etc.
	offset := int64(chunkIndex) * session.ChunkSize

	dst, err := os.OpenFile(session.FinalPath, os.O_WRONLY, 0644)
	if err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Failed to open file for writing"})
		return
	}

	// Seek to correct position
	if _, err := dst.Seek(offset, io.SeekStart); err != nil {
		dst.Close()
		c.JSON(500, models.ErrorResponse{Error: "Failed to seek in file"})
		return
	}

	// ⚡ Use pooled buffer for zero-alloc copy
	buf := getBuffer()
	defer putBuffer(buf)

	written, err := io.CopyBuffer(dst, src, *buf)
	dst.Close()

	if err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Failed to write chunk"})
		return
	}

	// Thread-safe mark chunk as received
	chunksReceived := session.MarkChunkReceived(chunkIndex)
	progress := (chunksReceived * 100) / totalChunks

	fmt.Printf("📥 [%d/%d] chunk %d → offset %d (%dKB) | %d%%\n",
		chunksReceived, totalChunks, chunkIndex, offset, written/1024, progress)

	var fileID string
	// If all chunks received, finalize (file is already assembled!)
	if chunksReceived == totalChunks {
		fmt.Printf("🚀 All chunks received for %s - Finalizing...\n", uploadID[:8])

		var err error
		fileID, err = finalizeSparseUpload(uploadID, session)
		if err != nil {
			fmt.Printf("❌ Finalize error for %s: %v\n", uploadID, err)
			os.Remove(session.FinalPath)
			uploadSessions.Delete(uploadID)
			c.JSON(500, models.ErrorResponse{Error: "Failed to finalize upload"})
			return
		}
		fmt.Printf("✅ Upload complete: %s (%s)\n", session.FileName, utils.FormatFileSize(session.FileSize))
	}

	c.JSON(200, models.ChunkUploadResponse{
		ChunkIndex:     chunkIndex,
		TotalChunks:    totalChunks,
		ChunksReceived: chunksReceived,
		Progress:       progress,
		ChunkComplete:  false,
		Ready:          chunksReceived == totalChunks,
		FileID:         fileID,
	})
}

// finalizeSparseUpload - file is already assembled, just verify and save to DB
func finalizeSparseUpload(uploadID string, session *models.UploadSession) (string, error) {
	// Verify final file size
	fileInfo, err := os.Stat(session.FinalPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat final file: %w", err)
	}

	if fileInfo.Size() != session.FileSize {
		os.Remove(session.FinalPath)
		return "", fmt.Errorf("final file size mismatch: expected %d, got %d", session.FileSize, fileInfo.Size())
	}

	// Save to database
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filesCollection := database.DB.Collection("files")
	finalName := filepath.Base(session.FinalPath)
	fileDoc := models.File{
		UserID:      session.UserID,
		FileName:    session.FileName,
		FileSize:    session.FileSize,
		FileType:    session.MimeType,
		StoragePath: finalName,
		Description: &session.Description,
		IsPublic:    false,
		CreatedAt:   time.Now().UnixMilli(),
		UpdatedAt:   time.Now().UnixMilli(),
	}

	res, err := filesCollection.InsertOne(ctx, fileDoc)
	if err != nil {
		os.Remove(session.FinalPath)
		return "", fmt.Errorf("failed to insert file to database: %w", err)
	}

	fmt.Printf("💾 File saved: %s\n", finalName)
	uploadSessions.Delete(uploadID)
	return res.InsertedID.(primitive.ObjectID).Hex(), nil
}

// UploadProgressCheck returns upload progress
func UploadProgressCheck(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(401, models.ErrorResponse{Error: "Unauthorized"})
		return
	}

	uploadID := c.Param("uploadId")

	sessionInterface, exists := uploadSessions.Load(uploadID)
	if !exists {
		c.JSON(404, models.ErrorResponse{Error: "Upload session not found"})
		return
	}

	session := sessionInterface.(*models.UploadSession)

	if session.UserID != userID {
		c.JSON(403, models.ErrorResponse{Error: "Unauthorized"})
		return
	}

	chunksReceived := session.GetReceivedCount()
	progress := (chunksReceived * 100) / session.TotalChunks
	missingChunks := session.GetMissingChunks()

	c.JSON(200, models.UploadProgressResponse{
		UploadID:       uploadID,
		ChunksReceived: chunksReceived,
		TotalChunks:    session.TotalChunks,
		Progress:       progress,
		Complete:       chunksReceived == session.TotalChunks,
		MissingChunks:  missingChunks,
	})
}

// Upload handles single file upload with streaming
func Upload(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(401, models.ErrorResponse{Error: "Unauthorized"})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(400, models.ErrorResponse{Error: "No file provided"})
		return
	}

	description := c.PostForm("description")
	description = utils.SanitizeInput(description, 1000)

	// Ensure upload directory exists
	uploadsDir := filepath.Join("uploads")
	utils.EnsureUploadDir(uploadsDir)

	// Generate unique filename
	uniqueName := utils.GenerateUniqueFileName(file.Filename)
	destPath := filepath.Join(uploadsDir, uniqueName)

	// ⚡ Stream directly from multipart to file using pooled buffer
	src, err := file.Open()
	if err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Failed to read file"})
		return
	}
	defer src.Close()

	dst, err := os.Create(destPath)
	if err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Failed to create file"})
		return
	}

	buf := getBuffer()
	_, err = io.CopyBuffer(dst, src, *buf)
	putBuffer(buf)
	dst.Close()

	if err != nil {
		os.Remove(destPath)
		c.JSON(500, models.ErrorResponse{Error: "Failed to save file"})
		return
	}

	// Get actual file size
	fileInfo, _ := os.Stat(destPath)

	// Save to database
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filesCollection := database.DB.Collection("files")
	fileDoc := models.File{
		UserID:      userID.(string),
		FileName:    file.Filename,
		FileSize:    fileInfo.Size(),
		FileType:    file.Header.Get("Content-Type"),
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
		FileSize: fileInfo.Size(),
	})
}

// GetFiles returns user's files
func GetFiles(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(401, models.ErrorResponse{Error: "Unauthorized"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	usersCollection := database.DB.Collection("users")

	objID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(400, models.ErrorResponse{Error: "Invalid user ID format"})
		return
	}

	// Check if admin
	var user models.User
	err = usersCollection.FindOne(ctx, bson.M{"_id": objID}).Decode(&user)
	if err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Database error"})
		return
	}

	filesCollection := database.DB.Collection("files")
	var query bson.M

	if user.Role != "admin" {
		query = bson.M{"user_id": userID}
	} else {
		query = bson.M{}
	}

	opts := options.Find()
	opts.SetSort(bson.M{"created_at": -1})

	cursor, err := filesCollection.Find(ctx, query, opts)
	if err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Failed to fetch files"})
		return
	}
	defer cursor.Close(ctx)

	var files []models.File
	if err := cursor.All(ctx, &files); err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Failed to decode files"})
		return
	}

	c.JSON(200, files)
}

// DeleteFile deletes a file
func DeleteFile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(401, models.ErrorResponse{Error: "Unauthorized"})
		return
	}

	fileID := c.Param("id")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check user role
	usersCollection := database.DB.Collection("users")
	objID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(400, models.ErrorResponse{Error: "Invalid user ID format"})
		return
	}

	var user models.User
	err = usersCollection.FindOne(ctx, bson.M{"_id": objID}).Decode(&user)
	if err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Database error"})
		return
	}

	filesCollection := database.DB.Collection("files")

	// Find file
	var fileDoc models.File
	objFileID, err := primitive.ObjectIDFromHex(fileID)
	if err != nil {
		c.JSON(400, models.ErrorResponse{Error: "Invalid file ID format"})
		return
	}

	err = filesCollection.FindOne(ctx, bson.M{"_id": objFileID}).Decode(&fileDoc)
	if err == mongo.ErrNoDocuments {
		c.JSON(404, models.ErrorResponse{Error: "File not found"})
		return
	} else if err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Database error"})
		return
	}

	// Check ownership or admin
	if fileDoc.UserID != userID && user.Role != "admin" {
		c.JSON(403, models.ErrorResponse{Error: "Unauthorized"})
		return
	}

	// Delete file from disk
	uploadsDir := filepath.Join("uploads")
	filePath := filepath.Join(uploadsDir, fileDoc.StoragePath)
	os.Remove(filePath)

	// Delete from database
	_, err = filesCollection.DeleteOne(ctx, bson.M{"_id": objFileID})
	if err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Failed to delete file"})
		return
	}

	c.JSON(200, models.SuccessResponse{Success: true})
}

// DownloadFile downloads a file
func DownloadFile(c *gin.Context) {
	filename := c.Param("filename")

	uploadsDir := filepath.Join("uploads")
	targetPath := filepath.Join(uploadsDir, filename)

	// Security check - prevent path traversal
	absUploads, _ := filepath.Abs(uploadsDir)
	absTarget, _ := filepath.Abs(targetPath)
	if len(absTarget) < len(absUploads) || absTarget[:len(absUploads)] != absUploads {
		c.JSON(403, models.ErrorResponse{Error: "Access denied"})
		return
	}

	if !utils.PathExists(targetPath) {
		c.JSON(404, models.ErrorResponse{Error: "File not found"})
		return
	}

	c.File(targetPath)
}
