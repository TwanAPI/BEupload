package models

import (
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// User model
type User struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Email       string      `bson:"email" json:"email"`
	Username    string      `bson:"username,omitempty" json:"username,omitempty"`
	Password    string      `bson:"password" json:"-"`
	DisplayName string      `bson:"displayName" json:"displayName"`
	Bio         string      `bson:"bio" json:"bio"`
	AvatarURL   string      `bson:"avatar_url" json:"avatar_url"`
	Location    string      `bson:"location" json:"location"`
	Website     string      `bson:"website" json:"website"`
	Phone       string      `bson:"phone" json:"phone"`
	Role        string      `bson:"role" json:"role"` // "user", "admin", "moderator"
	SocialMedia SocialMedia `bson:"social_media" json:"social_media"`
	CreatedAt   int64       `bson:"created_at" json:"created_at"`
	UpdatedAt   int64       `bson:"updated_at" json:"updated_at"`
}

type SocialMedia struct {
	Twitter   string `bson:"twitter" json:"twitter"`
	GitHub    string `bson:"github" json:"github"`
	LinkedIn  string `bson:"linkedin" json:"linkedin"`
	Instagram string `bson:"instagram" json:"instagram"`
}

// File model
type File struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID      string    `bson:"user_id" json:"user_id"`
	FileName    string    `bson:"file_name" json:"file_name"`
	FileSize    int64     `bson:"file_size" json:"file_size"`
	FileType    string    `bson:"file_type" json:"file_type"`
	StoragePath string    `bson:"storage_path" json:"storage_path"`
	Description *string   `bson:"description" json:"description"`
	IsPublic    bool      `bson:"is_public" json:"is_public"`
	CreatedAt   int64     `bson:"created_at" json:"created_at"`
	UpdatedAt   int64     `bson:"updated_at" json:"updated_at"`
}

// UploadSession for chunked uploads - thread-safe with RWMutex
type UploadSession struct {
	UploadID       string
	UserID         string
	FileName       string
	Description    string
	MimeType       string
	FileSize       int64
	ChunkSize      int64            // Size of each chunk for sparse file write
	TotalChunks    int
	ReceivedChunks map[int]bool
	FinalPath      string           // Pre-allocated final file path
	Mu             sync.RWMutex     // Protects ReceivedChunks for concurrent access
	CreatedAt      time.Time
}

// MarkChunkReceived thread-safe chunk tracking
func (s *UploadSession) MarkChunkReceived(index int) int {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.ReceivedChunks[index] = true
	return len(s.ReceivedChunks)
}

// GetReceivedCount thread-safe count
func (s *UploadSession) GetReceivedCount() int {
	s.Mu.RLock()
	defer s.Mu.RUnlock()
	return len(s.ReceivedChunks)
}

// IsChunkReceived checks if a specific chunk was received
func (s *UploadSession) IsChunkReceived(index int) bool {
	s.Mu.RLock()
	defer s.Mu.RUnlock()
	return s.ReceivedChunks[index]
}

// GetMissingChunks returns list of missing chunk indices
func (s *UploadSession) GetMissingChunks() []int {
	s.Mu.RLock()
	defer s.Mu.RUnlock()
	missing := make([]int, 0)
	for i := 0; i < s.TotalChunks; i++ {
		if !s.ReceivedChunks[i] {
			missing = append(missing, i)
		}
	}
	return missing
}

// User stats
type UserStats struct {
	TotalFiles      int64         `json:"totalFiles"`
	TotalStorage    int64         `json:"totalStorage"`
	AverageFileSize float64       `json:"averageFileSize"`
	RecentFiles     []FilePreview `json:"recentFiles"`
}

type FilePreview struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	Type      string    `json:"type"`
	CreatedAt int64     `json:"created_at"`
}

// Request/Response DTOs

type SignUpRequest struct {
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required,min=6"`
	DisplayName string `json:"displayName"`
	Username    string `json:"username"`
}

type SignInRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" binding:"required"`
	NewPassword     string `json:"newPassword" binding:"required,min=6"`
}

type UpdateProfileRequest struct {
	DisplayName string       `json:"displayName"`
	Bio         string       `json:"bio"`
	AvatarURL   string       `json:"avatar_url"`
	Location    string       `json:"location"`
	Website     string       `json:"website"`
	Phone       string       `json:"phone"`
	SocialMedia *SocialMedia `json:"social_media"`
}

type SetRoleRequest struct {
	Role string `json:"role" binding:"required,oneof=user admin moderator"`
}

type AuthResponse struct {
	User  UserResponse `json:"user"`
	Token string       `json:"token"`
}

type UserResponse struct {
	ID          string      `json:"id"`
	Email       string      `json:"email"`
	DisplayName string      `json:"displayName"`
	Username    string      `json:"username"`
	Bio         string      `json:"bio"`
	AvatarURL   string      `json:"avatar_url"`
	Location    string      `json:"location"`
	Website     string      `json:"website"`
	Phone       string      `json:"phone"`
	Role        string      `json:"role"`
	SocialMedia SocialMedia `json:"social_media"`
	CreatedAt   int64       `json:"created_at"`
	IsAdmin     bool        `json:"isAdmin"`
}

type UploadResponse struct {
	ID       string `json:"id"`
	FileName string `json:"file_name"`
	FileSize int64  `json:"file_size"`
}

type ChunkUploadResponse struct {
	ChunkIndex     int  `json:"chunkIndex"`
	TotalChunks    int  `json:"totalChunks"`
	ChunksReceived int  `json:"chunksReceived"`
	Progress       int  `json:"progress"`
	ChunkComplete  bool   `json:"chunkComplete"`
	Ready          bool   `json:"ready"`
	FileID         string `json:"fileId,omitempty"`
}

type UploadProgressResponse struct {
	UploadID       string `json:"uploadId"`
	ChunksReceived int    `json:"chunksReceived"`
	TotalChunks    int    `json:"totalChunks"`
	Progress       int    `json:"progress"`
	Complete       bool   `json:"complete"`
	MissingChunks  []int  `json:"missingChunks"`
}

type UserDetailResponse struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"displayName"`
	Username    string    `json:"username"`
	Role        string    `json:"role"`
	IsAdmin     bool      `json:"isAdmin"`
	Bio         string    `json:"bio"`
	AvatarURL   string    `json:"avatar_url"`
	Location    string    `json:"location"`
	Website     string    `json:"website"`
	Phone       string    `json:"phone"`
	CreatedAt   int64     `json:"created_at"`
	UpdatedAt   int64     `json:"updated_at"`
	Stats       UserStats `json:"stats"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type SuccessResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}
