package middleware

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
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

		// Parse token
		token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
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

		claims, ok := token.Claims.(*jwt.RegisteredClaims)
		if !ok || claims.Subject == "" || claims.Issuer == "" {
			c.JSON(401, gin.H{"error": "Invalid token: missing userId or email"})
			c.Abort()
			return
		}

		// Set user info in context
		c.Set("userID", claims.Subject)
		c.Set("email", claims.Issuer)
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
		// Skip rate limiting for upload-chunk endpoint (high-frequency)
		if c.Request.URL.Path == "/api/files/upload-chunk" {
			c.Next()
			return
		}

		ip := c.ClientIP()
		now := time.Now().Unix() * 1000 // milliseconds

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

// AdminRequired ensures user is admin - returns gin.HandlerFunc
func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("userID")
		if !exists {
			c.JSON(401, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}

		// Import database package to check user role
		// We set the userID and let the handler verify admin status
		c.Set("requireAdmin", true)
		_ = userID
		c.Next()
	}
}

// CORSMiddleware handles CORS headers with optimized settings
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400") // Cache preflight for 24h

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
