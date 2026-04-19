package middleware

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"cloud-caddy-backend/internal/database"
	"cloud-caddy-backend/internal/utils"
)

var JWTSecret string

// SetJWTSecret sets the JWT secret key
func SetJWTSecret(secret string) {
	JWTSecret = secret
}

// JWTAuth middleware verifies JWT token
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(401, gin.H{"error": "No token provided"})
			c.Abort()
			return
		}

		// Extract token from "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(401, gin.H{"error": "Invalid token format"})
			c.Abort()
			return
		}

		tokenString := parts[1]

		// Parse token with custom claims
		token, err := jwt.ParseWithClaims(tokenString, &utils.CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
			// Verify signing method
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(JWTSecret), nil
		})

		if err != nil || !token.Valid {
			c.JSON(401, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(*utils.CustomClaims)
		if !ok || claims.Subject == "" {
			c.JSON(401, gin.H{"error": "Invalid token claims"})
			c.Abort()
			return
		}

		// Verify issuer
		if claims.Issuer != "cloud-caddy" {
			c.JSON(401, gin.H{"error": "Invalid token issuer"})
			c.Abort()
			return
		}

		// Set user info in context
		c.Set("userID", claims.Subject)
		c.Set("email", claims.Email)
		c.Next()
	}
}

// rateLimitEntry stores per-IP rate limit data
type rateLimitEntry struct {
	count       int
	windowStart int64
}

// RateLimitMiddleware - thread-safe rate limiting using sync.Map
func RateLimitMiddleware(maxRequests int, windowMs int64) gin.HandlerFunc {
	var ipLimits sync.Map

	// Periodic cleanup of stale entries (every 5 min)
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		for range ticker.C {
			now := time.Now().Unix() * 1000
			ipLimits.Range(func(key, value interface{}) bool {
				entry := value.(*rateLimitEntry)
				if now-entry.windowStart > windowMs*2 {
					ipLimits.Delete(key)
				}
				return true
			})
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now().Unix() * 1000 // milliseconds

		// Apply separate higher limit for upload-chunk (still not unlimited)
		if c.Request.URL.Path == "/api/files/upload-chunk" {
			// Allow 50000 chunk uploads per minute per IP (high but not unlimited)
			chunkLimitKey := ip + ":chunk"
			chunkVal, chunkLoaded := ipLimits.LoadOrStore(chunkLimitKey, &rateLimitEntry{count: 1, windowStart: now})
			if !chunkLoaded {
				c.Next()
				return
			}
			chunkEntry := chunkVal.(*rateLimitEntry)
			if now-chunkEntry.windowStart > windowMs {
				chunkEntry.count = 1
				chunkEntry.windowStart = now
				c.Next()
				return
			}
			chunkEntry.count++
			if chunkEntry.count > 50000 {
				c.JSON(429, gin.H{"error": "Upload rate limit exceeded"})
				c.Abort()
				return
			}
			c.Next()
			return
		}

		val, loaded := ipLimits.LoadOrStore(ip, &rateLimitEntry{count: 1, windowStart: now})
		if !loaded {
			c.Next()
			return
		}

		entry := val.(*rateLimitEntry)

		// Reset window if expired
		if now-entry.windowStart > windowMs {
			entry.count = 1
			entry.windowStart = now
			c.Next()
			return
		}

		entry.count++
		if entry.count > maxRequests {
			c.JSON(429, gin.H{"error": "Too many requests, please try again later"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// AdminRequired ensures user is admin - checks role from database
func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("userID")
		if !exists {
			c.JSON(401, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}

		// Check user role from database
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		usersCollection := database.DB.Collection("users")

		objID, err := primitive.ObjectIDFromHex(userID.(string))
		if err != nil {
			c.JSON(403, gin.H{"error": "Invalid user ID"})
			c.Abort()
			return
		}

		var user struct {
			Role string `bson:"role"`
		}
		err = usersCollection.FindOne(ctx, bson.M{"_id": objID}).Decode(&user)
		if err != nil {
			c.JSON(403, gin.H{"error": "User not found"})
			c.Abort()
			return
		}

		if user.Role != "admin" {
			c.JSON(403, gin.H{"error": "Admin access required"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// CORSMiddleware handles CORS headers with security-conscious settings
func CORSMiddleware() gin.HandlerFunc {
	// Allowed origins — configure for production
	allowedOrigins := map[string]bool{
		"http://localhost:8080":   true,
		"http://localhost:3001":   true,
		"http://localhost:5173":   true,
		"http://127.0.0.1:8080":   true,
		"http://127.0.0.1:5173":   true,
		"http://10.104.0.4:8080":  true,
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		if allowedOrigins[origin] {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		} else if origin == "" {
			// Same-origin requests (no Origin header) — allow
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		}

		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
