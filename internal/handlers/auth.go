package handlers

import (
	"context"
	"net/mail"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"

	"cloud-caddy-backend/internal/database"
	"cloud-caddy-backend/internal/models"
	"cloud-caddy-backend/internal/utils"
)

// SignUp handles user registration
func SignUp(c *gin.Context) {
	var req models.SignUpRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, models.ErrorResponse{Error: "Invalid request"})
		return
	}

	// Validate email format
	if _, err := mail.ParseAddress(req.Email); err != nil {
		c.JSON(400, models.ErrorResponse{Error: "Invalid email format"})
		return
	}

	// Sanitize inputs
	email := utils.SanitizeInput(req.Email, 255)
	password := utils.SanitizeInput(req.Password, 256)
	displayName := utils.SanitizeInput(req.DisplayName, 100)
	username := utils.SanitizeInput(req.Username, 20)

	// Validate username if provided
	if username != "" {
		if !regexp.MustCompile(`^[a-zA-Z0-9_]{3,20}$`).MatchString(username) {
			c.JSON(400, models.ErrorResponse{Error: "Username must be 3-20 characters (letters, numbers, underscore only)"})
			return
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	usersCollection := database.DB.Collection("users")

	// Check if email exists
	var existingUser models.User
	err := usersCollection.FindOne(ctx, bson.M{"email": email}).Decode(&existingUser)
	if err == nil {
		c.JSON(400, models.ErrorResponse{Error: "Email already exists"})
		return
	} else if err != mongo.ErrNoDocuments {
		c.JSON(500, models.ErrorResponse{Error: "Database error"})
		return
	}

	// Check if username exists
	if username != "" {
		err := usersCollection.FindOne(ctx, bson.M{"username": username}).Decode(&existingUser)
		if err == nil {
			c.JSON(400, models.ErrorResponse{Error: "Username already taken"})
			return
		} else if err != mongo.ErrNoDocuments {
			c.JSON(500, models.ErrorResponse{Error: "Database error"})
			return
		}
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Failed to hash password"})
		return
	}

	// Create user
	if displayName == "" {
		displayName = email[:len(email)-len("@"+email[len(email)-4:])]
	}

	user := models.User{
		Email:       email,
		Password:    string(hashedPassword),
		Username:    username,
		DisplayName: displayName,
		Bio:         "",
		Role:        "user",
		SocialMedia: models.SocialMedia{},
		CreatedAt:   time.Now().UnixMilli(),
		UpdatedAt:   time.Now().UnixMilli(),
	}

	result, err := usersCollection.InsertOne(ctx, user)
	if err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Failed to create user"})
		return
	}

	userIDStr := result.InsertedID.(primitive.ObjectID).Hex()
	user.ID = result.InsertedID.(primitive.ObjectID)

	// Generate JWT
	token, err := utils.GenerateJWT(userIDStr, email, c.GetString("jwtSecret"))
	if err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Failed to generate token"})
		return
	}

	c.JSON(200, models.AuthResponse{
		User: models.UserResponse{
			ID:          userIDStr,
			Email:       email,
			DisplayName: displayName,
			Username:    username,
		},
		Token: token,
	})
}

// SignIn handles user login
func SignIn(c *gin.Context) {
	var req models.SignInRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, models.ErrorResponse{Error: "Invalid request"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	usersCollection := database.DB.Collection("users")

	// Find user by email
	var user models.User
	err := usersCollection.FindOne(ctx, bson.M{"email": req.Email}).Decode(&user)
	if err == mongo.ErrNoDocuments {
		c.JSON(400, models.ErrorResponse{Error: "Invalid credentials"})
		return
	} else if err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Database error"})
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(400, models.ErrorResponse{Error: "Invalid credentials"})
		return
	}

	// Generate JWT
	token, err := utils.GenerateJWT(user.ID.Hex(), user.Email, c.GetString("jwtSecret"))
	if err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Failed to generate token"})
		return
	}

	c.JSON(200, gin.H{
		"message": "Signed in successfully",
		"token":   token,
		"user": models.UserResponse{
			ID:          user.ID.Hex(),
			Email:       user.Email,
			DisplayName: user.DisplayName,
			Username:    user.Username,
		},
	})
}

// GetMe returns current user information
func GetMe(c *gin.Context) {
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

	var user models.User
	err = usersCollection.FindOne(ctx, bson.M{"_id": objID}).Decode(&user)
	if err == mongo.ErrNoDocuments {
		c.JSON(404, models.ErrorResponse{Error: "User not found"})
		return
	} else if err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Database error"})
		return
	}

	c.JSON(200, models.UserResponse{
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

// ChangePassword changes user password
func ChangePassword(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(401, models.ErrorResponse{Error: "Unauthorized"})
		return
	}

	var req models.ChangePasswordRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, models.ErrorResponse{Error: "Invalid request"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	usersCollection := database.DB.Collection("users")

	objID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(400, models.ErrorResponse{Error: "Invalid user ID"})
		return
	}

	// Find user
	var user models.User
	err = usersCollection.FindOne(ctx, bson.M{"_id": objID}).Decode(&user)
	if err == mongo.ErrNoDocuments {
		c.JSON(404, models.ErrorResponse{Error: "User not found"})
		return
	} else if err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Database error"})
		return
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.CurrentPassword)); err != nil {
		c.JSON(401, models.ErrorResponse{Error: "Current password is incorrect"})
		return
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), 10)
	if err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Failed to hash password"})
		return
	}

	// Update password
	_, err = usersCollection.UpdateOne(
		ctx,
		bson.M{"_id": objID},
		bson.M{"$set": bson.M{
			"password":   string(hashedPassword),
			"updated_at": time.Now().UnixMilli(),
		}},
	)
	if err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Failed to update password"})
		return
	}

	c.JSON(200, models.SuccessResponse{Success: true, Message: "Password changed successfully"})
}

// UpdateProfile updates user profile
func UpdateProfile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(401, models.ErrorResponse{Error: "Unauthorized"})
		return
	}

	var req models.UpdateProfileRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, models.ErrorResponse{Error: "Invalid request"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	usersCollection := database.DB.Collection("users")

	// Build update map
	updates := bson.M{
		"updated_at": time.Now().UnixMilli(),
	}

	if req.DisplayName != "" {
		updates["displayName"] = req.DisplayName
		updates["display_name"] = req.DisplayName
	}
	if req.Bio != "" {
		updates["bio"] = req.Bio
	}
	if req.AvatarURL != "" {
		updates["avatar_url"] = req.AvatarURL
	}
	if req.Location != "" {
		updates["location"] = req.Location
	}
	if req.Website != "" {
		updates["website"] = req.Website
	}
	if req.Phone != "" {
		updates["phone"] = req.Phone
	}
	if req.SocialMedia != nil {
		updates["social_media"] = req.SocialMedia
	}

	objID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(400, models.ErrorResponse{Error: "Invalid user ID"})
		return
	}

	// Update user
	_, err = usersCollection.UpdateOne(
		ctx,
		bson.M{"_id": objID},
		bson.M{"$set": updates},
	)
	if err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Failed to update profile"})
		return
	}

	// Return updated user
	var user models.User
	err = usersCollection.FindOne(ctx, bson.M{"_id": objID}).Decode(&user)
	if err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Failed to fetch updated user"})
		return
	}

	c.JSON(200, models.UserResponse{
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
