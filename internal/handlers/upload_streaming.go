package handlers\r
\r
import (\r
	"context"\r
	"fmt"\r
	"io"\r
	"os"\r
	"path/filepath"\r
	"strconv"\r
	"strings"\r
	"time"\r
\r
	"github.com/gin-gonic/gin"\r
	"go.mongodb.org/mongo-driver/bson/primitive"\r
\r
	"cloud-caddy-backend/internal/database"\r
	"cloud-caddy-backend/internal/models"\r
	"cloud-caddy-backend/internal/utils"\r
)\r
\r
// UploadStream handles streaming file upload via PUT request\r
// Reads directly from request body — NO multipart parsing overhead\r
// Metadata passed via HTTP headers:\r
//   X-File-Name: original filename\r
//   X-File-Size: file size in bytes (optional, for pre-allocation)\r
//   X-Description: file description (optional)\r
func UploadStream(c *gin.Context) {\r
	// ⚡ Concurrency control: reject if server at capacity\r
	if !acquireUploadSlot() {\r
		c.JSON(503, models.ErrorResponse{Error: "Server at upload capacity, please retry"})\r
		return\r
	}\r
	defer releaseUploadSlot()\r
\r
	// 1. Get userID from gin context (set by JWTAuth middleware)\r
	userID, exists := c.Get("userID")\r
	if !exists {\r
		c.JSON(401, models.ErrorResponse{Error: "Unauthorized"})\r
		return\r
	}\r
\r
	// 2. Read metadata from headers\r
	fileName := strings.TrimSpace(c.GetHeader("X-File-Name"))\r
	fileSizeStr := strings.TrimSpace(c.GetHeader("X-File-Size"))\r
	description := c.GetHeader("X-Description")\r
\r
	// 3. Validate X-File-Name is not empty, sanitize it\r
	if fileName == "" {\r
		c.JSON(400, models.ErrorResponse{Error: "X-File-Name header is required"})\r
		return\r
	}\r
	fileName = filepath.Base(fileName)\r
	fileName = utils.SanitizeInput(fileName, 255)\r
	if fileName == "" || fileName == "." || fileName == "/" || fileName == "\\" {\r
		c.JSON(400, models.ErrorResponse{Error: "Invalid X-File-Name header"})\r
		return\r
	}\r
\r
	description = utils.SanitizeInput(description, 1000)\r
\r
	// 4. If X-File-Size provided, validate it's > 0 and <= 10GB\r
	var fileSize int64\r
	if fileSizeStr != "" {\r
		parsedSize, err := strconv.ParseInt(fileSizeStr, 10, 64)\r
		if err != nil || parsedSize <= 0 || parsedSize > maxFileSize {\r
			c.JSON(400, models.ErrorResponse{\r
				Error: fmt.Sprintf("File size must be between 1 byte and %s", utils.FormatFileSize(maxFileSize)),\r
			})\r
			return\r
		}\r
		fileSize = parsedSize\r
	}\r
\r
	// Verify request body is present\r
	if c.Request.Body == nil {\r
		c.JSON(400, models.ErrorResponse{Error: "Request body cannot be empty"})\r
		return\r
	}\r
\r
	// 5. Ensure upload dir exists\r
	uploadsDir := filepath.Join("uploads")\r
	if err := utils.EnsureUploadDir(uploadsDir); err != nil {\r
		c.JSON(500, models.ErrorResponse{Error: "Failed to create upload directory"})\r
		return\r
	}\r
\r
	// 6. Generate unique filename\r
	uniqueName := utils.GenerateUniqueFileName(fileName)\r
	destPath := filepath.Join(uploadsDir, uniqueName)\r
\r
	// 7. Create destination file. If fileSize known, pre-allocate with f.Truncate(fileSize)\r
	dst, err := os.Create(destPath)\r
	if err != nil {\r
		c.JSON(500, models.ErrorResponse{Error: "Failed to create file"})\r
		return\r
	}\r
\r
	if fileSize > 0 {\r
		if err := dst.Truncate(fileSize); err != nil {\r
			dst.Close()\r
			os.Remove(destPath)\r
			c.JSON(500, models.ErrorResponse{Error: "Failed to allocate file"})\r
			return\r
		}\r
		if _, err := dst.Seek(0, io.SeekStart); err != nil {\r
			dst.Close()\r
			os.Remove(destPath)\r
			c.JSON(500, models.ErrorResponse{Error: "Failed to seek file"})\r
			return\r
		}\r
	}\r
\r
	// 8. Stream directly from c.Request.Body to file using io.CopyBuffer with a 32MB pooled buffer\r
	buf := getBuffer()\r
	defer putBuffer(buf)\r
\r
	written, copyErr := io.CopyBuffer(dst, c.Request.Body, *buf)\r
	dst.Close()\r
\r
	if copyErr != nil {\r
		os.Remove(destPath)\r
		c.JSON(500, models.ErrorResponse{Error: "Failed to save file"})\r
		return\r
	}\r
\r
	// 9. After copy, if fileSize was provided, verify actual written size matches\r
	if fileSize > 0 && written != fileSize {\r
		os.Remove(destPath)\r
		c.JSON(400, models.ErrorResponse{\r
			Error: fmt.Sprintf("File size mismatch: expected %d bytes, got %d bytes", fileSize, written),\r
		})\r
		return\r
	}\r
\r
	if written <= 0 {\r
		os.Remove(destPath)\r
		c.JSON(400, models.ErrorResponse{Error: "Uploaded file is empty"})\r
		return\r
	}\r
\r
	if written > maxFileSize {\r
		os.Remove(destPath)\r
		c.JSON(400, models.ErrorResponse{\r
			Error: fmt.Sprintf("File size must be between 1 byte and %s", utils.FormatFileSize(maxFileSize)),\r
		})\r
		return\r
	}\r
\r
	// 10. Get actual file size via os.Stat()\r
	fileInfo, err := os.Stat(destPath)\r
	if err != nil {\r
		os.Remove(destPath)\r
		c.JSON(500, models.ErrorResponse{Error: "Failed to stat file"})\r
		return\r
	}\r
	actualSize := fileInfo.Size()\r
\r
	if fileSize > 0 && actualSize != fileSize {\r
		os.Remove(destPath)\r
		c.JSON(400, models.ErrorResponse{\r
			Error: fmt.Sprintf("File size mismatch: expected %d bytes, got %d bytes", fileSize, actualSize),\r
		})\r
		return\r
	}\r
\r
	// 11. Save metadata to MongoDB files collection (same as Upload handler in files.go)\r
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)\r
	defer cancel()\r
\r
	filesCollection := database.DB.Collection("files")\r
	fileDoc := models.File{\r
		UserID:      userID.(string),\r
		FileName:    fileName,\r
		FileSize:    actualSize,\r
		FileType:    c.GetHeader("Content-Type"),\r
		StoragePath: uniqueName,\r
		Description: &description,\r
		IsPublic:    false,\r
		CreatedAt:   time.Now().UnixMilli(),\r
		UpdatedAt:   time.Now().UnixMilli(),\r
	}\r
\r
	result, err := filesCollection.InsertOne(ctx, fileDoc)\r
	if err != nil {\r
		os.Remove(destPath)\r
		c.JSON(500, models.ErrorResponse{Error: "Failed to save file metadata"})\r
		return\r
	}\r
\r
	// 12. Return same response format as Upload: models.UploadResponse{ID, FileName, FileSize}\r
	c.JSON(200, models.UploadResponse{\r
		ID:       result.InsertedID.(primitive.ObjectID).Hex(),\r
		FileName: fileDoc.FileName,\r
		FileSize: actualSize,\r
	})\r
}\r
