package handlers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"cloud-caddy-backend/internal/database"
	"cloud-caddy-backend/internal/models"
)

// GetAllUsers returns all users (admin only)
func GetAllUsers(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(401, models.ErrorResponse{Error: "Unauthorized"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	usersCollection := database.DB.Collection("users")

	// Verify admin
	var admin models.User
	objID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(403, models.ErrorResponse{Error: "Unauthorized"})
		return
	}
	err = usersCollection.FindOne(ctx, bson.M{"_id": objID}).Decode(&admin)
	if err != nil || admin.Role != "admin" {
		c.JSON(403, models.ErrorResponse{Error: "Unauthorized"})
		return
	}

	// Get all users without password
	opts := options.Find()
	opts.SetProjection(bson.M{"password": 0})

	cursor, err := usersCollection.Find(ctx, bson.M{}, opts)
	if err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Failed to fetch users"})
		return
	}
	defer cursor.Close(ctx)

	var users []models.User
	if err := cursor.All(ctx, &users); err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Failed to decode users"})
		return
	}

	// Format response
	var responses []models.UserResponse
	for _, user := range users {
		responses = append(responses, models.UserResponse{
			ID:          user.ID.Hex(),
			Email:       user.Email,
			DisplayName: user.DisplayName,
			Username:    user.Username,
			Bio:         user.Bio,
			AvatarURL:   user.AvatarURL,
			Location:    user.Location,
			Website:     user.Website,
			Phone:       user.Phone,
			Role:        user.Role,
			SocialMedia: user.SocialMedia,
			CreatedAt:   user.CreatedAt,
			IsAdmin:     user.Role == "admin",
		})
	}

	c.JSON(200, responses)
}

// GetAdminFiles returns all files (admin only)
func GetAdminFiles(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(401, models.ErrorResponse{Error: "Unauthorized"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	usersCollection := database.DB.Collection("users")

	// Verify admin
	var admin models.User
	objID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(403, models.ErrorResponse{Error: "Unauthorized"})
		return
	}
	err = usersCollection.FindOne(ctx, bson.M{"_id": objID}).Decode(&admin)
	if err != nil || admin.Role != "admin" {
		c.JSON(403, models.ErrorResponse{Error: "Unauthorized"})
		return
	}

	filesCollection := database.DB.Collection("files")
	opts := options.Find()
	opts.SetSort(bson.M{"created_at": -1})

	cursor, err := filesCollection.Find(ctx, bson.M{}, opts)
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

	var fileResponses []gin.H
	for _, f := range files {
		fileResponses = append(fileResponses, gin.H{
			"id":         f.ID.Hex(),
			"filename":   f.FileName,
			"filesize":   f.FileSize,
			"filetype":   f.FileType,
			"created_at": f.CreatedAt,
			"user_id":    f.UserID,
		})
	}

	c.JSON(200, fileResponses)
}

// SetUserRole sets user role (admin only)
func SetUserRole(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(401, models.ErrorResponse{Error: "Unauthorized"})
		return
	}

	targetUserID := c.Param("userId")

	var req models.SetRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, models.ErrorResponse{Error: "Invalid request"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	usersCollection := database.DB.Collection("users")

	// Verify admin
	var admin models.User
	objID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(403, models.ErrorResponse{Error: "Unauthorized"})
		return
	}
	err = usersCollection.FindOne(ctx, bson.M{"_id": objID}).Decode(&admin)
	if err != nil || admin.Role != "admin" {
		c.JSON(403, models.ErrorResponse{Error: "Unauthorized"})
		return
	}

	objTargetID, err := primitive.ObjectIDFromHex(targetUserID)
	if err != nil {
		c.JSON(400, models.ErrorResponse{Error: "Invalid target user ID"})
		return
	}

	// Update user role
	_, err = usersCollection.UpdateOne(
		ctx,
		bson.M{"_id": objTargetID},
		bson.M{"$set": bson.M{
			"role":       req.Role,
			"updated_at": time.Now().UnixMilli(),
		}},
	)
	if err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Failed to update role"})
		return
	}

	c.JSON(200, gin.H{"success": true, "role": req.Role})
}

// GetUserDetail returns detailed user info with stats (admin only)
func GetUserDetail(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(401, models.ErrorResponse{Error: "Unauthorized"})
		return
	}

	targetUserID := c.Param("userId")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	usersCollection := database.DB.Collection("users")

	// Verify admin
	var admin models.User
	objID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(403, models.ErrorResponse{Error: "Unauthorized"})
		return
	}
	err = usersCollection.FindOne(ctx, bson.M{"_id": objID}).Decode(&admin)
	if err != nil || admin.Role != "admin" {
		c.JSON(403, models.ErrorResponse{Error: "Unauthorized"})
		return
	}

	// Get target user
	var targetUser models.User
	objTargetID, err := primitive.ObjectIDFromHex(targetUserID)
	if err != nil {
		c.JSON(400, models.ErrorResponse{Error: "Invalid target user ID"})
		return
	}
	err = usersCollection.FindOne(ctx, bson.M{"_id": objTargetID}).Decode(&targetUser)
	if err == mongo.ErrNoDocuments {
		c.JSON(404, models.ErrorResponse{Error: "User not found"})
		return
	} else if err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Database error"})
		return
	}

	// Get user's files
	filesCollection := database.DB.Collection("files")
	cursor, err := filesCollection.Find(ctx, bson.M{"user_id": targetUserID})
	if err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Failed to fetch files"})
		return
	}
	defer cursor.Close(ctx)

	var userFiles []models.File
	if err := cursor.All(ctx, &userFiles); err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Failed to decode files"})
		return
	}

	// Calculate stats
	totalStorage := int64(0)
	for _, f := range userFiles {
		totalStorage += f.FileSize
	}

	averageSize := 0.0
	if len(userFiles) > 0 {
		averageSize = float64(totalStorage) / float64(len(userFiles))
	}

	// Get recent files (last 5)
	recentFiles := make([]models.FilePreview, 0)
	for i, f := range userFiles {
		if i >= 5 {
			break
		}
		recentFiles = append(recentFiles, models.FilePreview{
			ID:        f.ID.Hex(),
			Name:      f.FileName,
			Size:      f.FileSize,
			Type:      f.FileType,
			CreatedAt: f.CreatedAt,
		})
	}

	response := models.UserDetailResponse{
		ID:          targetUser.ID.Hex(),
		Email:       targetUser.Email,
		DisplayName: targetUser.DisplayName,
		Username:    targetUser.Username,
		Role:        targetUser.Role,
		IsAdmin:     targetUser.Role == "admin",
		Bio:         targetUser.Bio,
		AvatarURL:   targetUser.AvatarURL,
		Location:    targetUser.Location,
		Website:     targetUser.Website,
		Phone:       targetUser.Phone,
		CreatedAt:   targetUser.CreatedAt,
		UpdatedAt:   targetUser.UpdatedAt,
		Stats: models.UserStats{
			TotalFiles:      int64(len(userFiles)),
			TotalStorage:    totalStorage,
			AverageFileSize: averageSize,
			RecentFiles:     recentFiles,
		},
	}

	c.JSON(200, response)
}

// GetUserFiles returns all user files (admin only)
func GetUserFiles(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(401, models.ErrorResponse{Error: "Unauthorized"})
		return
	}

	targetUserID := c.Param("userId")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	usersCollection := database.DB.Collection("users")

	// Verify admin
	var admin models.User
	objID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(403, models.ErrorResponse{Error: "Unauthorized"})
		return
	}
	err = usersCollection.FindOne(ctx, bson.M{"_id": objID}).Decode(&admin)
	if err != nil || admin.Role != "admin" {
		c.JSON(403, models.ErrorResponse{Error: "Unauthorized"})
		return
	}

	// Get user's files
	filesCollection := database.DB.Collection("files")
	opts := options.Find()
	opts.SetSort(bson.M{"created_at": -1})

	cursor, err := filesCollection.Find(ctx, bson.M{"user_id": targetUserID}, opts)
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

// DeleteAdminFile deletes a file (admin only)
func DeleteAdminFile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(401, models.ErrorResponse{Error: "Unauthorized"})
		return
	}

	fileID := c.Param("fileId")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	usersCollection := database.DB.Collection("users")

	// Verify admin
	var admin models.User
	objID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(403, models.ErrorResponse{Error: "Unauthorized"})
		return
	}
	err = usersCollection.FindOne(ctx, bson.M{"_id": objID}).Decode(&admin)
	if err != nil || admin.Role != "admin" {
		c.JSON(403, models.ErrorResponse{Error: "Unauthorized"})
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

	c.JSON(200, models.SuccessResponse{Success: true, Message: "File deleted successfully"})
}

// DeleteAllUserFiles deletes all files for a user (admin only)
func DeleteAllUserFiles(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(401, models.ErrorResponse{Error: "Unauthorized"})
		return
	}

	targetUserID := c.Param("userId")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	usersCollection := database.DB.Collection("users")

	// Verify admin
	var admin models.User
	objID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(403, models.ErrorResponse{Error: "Unauthorized"})
		return
	}
	err = usersCollection.FindOne(ctx, bson.M{"_id": objID}).Decode(&admin)
	if err != nil || admin.Role != "admin" {
		c.JSON(403, models.ErrorResponse{Error: "Unauthorized"})
		return
	}

	filesCollection := database.DB.Collection("files")

	// Get all user files
	cursor, err := filesCollection.Find(ctx, bson.M{"user_id": targetUserID})
	if err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Failed to fetch files"})
		return
	}
	defer cursor.Close(ctx)

	var userFiles []models.File
	if err := cursor.All(ctx, &userFiles); err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Failed to decode files"})
		return
	}

	// Delete files from disk
	uploadsDir := filepath.Join("uploads")
	for _, file := range userFiles {
		filePath := filepath.Join(uploadsDir, file.StoragePath)
		os.Remove(filePath)
	}

	// Delete all from database
	result, err := filesCollection.DeleteMany(ctx, bson.M{"user_id": targetUserID})
	if err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Failed to delete files"})
		return
	}

	c.JSON(200, gin.H{
		"success":      true,
		"message":      fmt.Sprintf("Deleted %d files", result.DeletedCount),
		"deletedCount": result.DeletedCount,
	})
}

// HealthCheck returns health status
func HealthCheck(c *gin.Context) {
	c.JSON(200, gin.H{"status": "ok"})
}

// HealthCheckStats returns health status with stats
func HealthCheckStats(c *gin.Context) {
	activeSessions := 0
	uploadSessions.Range(func(key, value interface{}) bool {
		activeSessions++
		return true
	})

	c.JSON(200, gin.H{
		"status": "ok",
		"uptime": time.Now().Unix(),
		"uploads": gin.H{
			"activeSessions": activeSessions,
			"concurrency": gin.H{
				"maxConcurrentFiles":       8,
				"chunkConcurrencyStrategy": "adaptive",
				"sparseFileEnabled":        true,
				"bufferPoolSize":           "32MB",
			},
		},
	})
}
