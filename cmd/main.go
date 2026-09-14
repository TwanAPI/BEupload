package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"cloud-caddy-backend/internal/database"
	"cloud-caddy-backend/internal/handlers"
	"cloud-caddy-backend/internal/middleware"
	"cloud-caddy-backend/internal/utils"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(".env.server"); err != nil {
		log.Println("⚠️  No .env.server file found, using environment variables")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "3001"
	}

	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		mongoURI = "mongodb+srv://admin:Tuandzvcl@userupload.1zqgqci.mongodb.net/?appName=UserUpload"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "TwanDz"
	}

	// Initialize MongoDB
	fmt.Println("🔄 Connecting to MongoDB...")
	if err := database.Connect(mongoURI); err != nil {
		log.Fatalf("❌ Failed to connect to MongoDB: %v", err)
	}
	defer database.Disconnect()

	// Set JWT secret globally
	middleware.SetJWTSecret(jwtSecret)

	// ⚡ Performance: use all CPU cores
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Create Gin router with performance mode
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// ⚡ Reduced from 256MB → 32MB: chunks are typically 5-10MB
	// 256MB per request was causing massive RAM usage under concurrency
	router.MaxMultipartMemory = 32 << 20 // 32 MB

	// Middleware
	router.Use(middleware.CORSMiddleware())
	router.Use(middleware.RateLimitMiddleware(10000, 60*1000)) // 10000 req/min for ultra-fast uploads

	// Middleware to set JWT secret in context
	router.Use(func(c *gin.Context) {
		c.Set("jwtSecret", jwtSecret)
		c.Next()
	})

	// Health check routes (no auth required)
	router.GET("/health", handlers.HealthCheck)
	router.GET("/health/stats", handlers.HealthCheckStats)

	// Auth routes (no auth required)
	authGroup := router.Group("/auth")
	{
		authGroup.POST("/signup", handlers.SignUp)
		authGroup.POST("/signin", handlers.SignIn)
		authGroup.GET("/me", middleware.JWTAuth(), handlers.GetMe)
		authGroup.POST("/change-password", middleware.JWTAuth(), handlers.ChangePassword)
		authGroup.POST("/update-profile", middleware.JWTAuth(), handlers.UpdateProfile)
	}

	// File routes (auth required)
	fileGroup := router.Group("/api/files")
	fileGroup.Use(middleware.JWTAuth())
	{
		fileGroup.POST("/upload", handlers.Upload)
		fileGroup.PUT("/upload-stream", handlers.UploadStream) // ⚡ Zero-copy streaming upload
		fileGroup.POST("/upload-chunk", handlers.UploadChunk)
		fileGroup.GET("/upload-progress/:uploadId", handlers.UploadProgressCheck)
		fileGroup.GET("", handlers.GetFiles)
		fileGroup.DELETE("/:id", handlers.DeleteFile)
		fileGroup.GET("/download/:filename", handlers.DownloadFile)
	}

	// Public file access (no auth - for sharing links)
	router.GET("/uploads/:filename", handlers.DownloadFile)

	// Admin routes (admin auth required)
	adminGroup := router.Group("/api/admin")
	adminGroup.Use(middleware.JWTAuth(), middleware.AdminRequired())
	{
		// User management
		adminGroup.GET("/users", handlers.GetAllUsers)
		adminGroup.GET("/users/:userId/detail", handlers.GetUserDetail)
		adminGroup.GET("/users/:userId/files", handlers.GetUserFiles)
		adminGroup.POST("/users/:userId/role", handlers.SetUserRole)
		adminGroup.DELETE("/users/:userId/files", handlers.DeleteAllUserFiles)

		// File management
		adminGroup.POST("/users/:userId/delete-all-files", handlers.DeleteAllUserFiles)
		adminGroup.GET("/files", handlers.GetAdminFiles)
		adminGroup.DELETE("/files/:fileId", handlers.DeleteAdminFile)
	}

	// Cleanup temporary chunks periodically (every 30 min instead of 1hr)
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		for range ticker.C {
			uploadsDir := "uploads"
			utils.CleanupOldTempChunks(uploadsDir, 1*time.Hour)
		}
	}()

	// ⚡ Custom HTTP server with proper timeouts
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      0,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	fmt.Printf("🚀 Server starting on http://localhost:%s\n", port)
	fmt.Printf("⚡ GOMAXPROCS=%d | MaxMultipartMemory=32MB | IdleTimeout=120s\n", runtime.NumCPU())
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("❌ Server failed to start: %v", err)
	}
}
